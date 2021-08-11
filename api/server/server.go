package server

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/andres-teleport/overseer/api"
	"github.com/andres-teleport/overseer/api/authentication"
	"github.com/andres-teleport/overseer/lib/multipipe"
	"github.com/andres-teleport/overseer/lib/supervisor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrEmptyCommand = status.Error(codes.InvalidArgument, "empty job command provided")
)

type Server struct {
	jobOwners  map[string]string
	mu         *sync.RWMutex
	supervisor *supervisor.Supervisor
	srv        *grpc.Server
	l          net.Listener
	api.UnimplementedJobworkerServiceServer
}

func NewServer(listenAddr, keyFile, certFile, caFile string) (*Server, error) {
	creds, err := authentication.NewServerTransportCredentials(keyFile, certFile, caFile)
	if err != nil {
		return nil, err
	}

	s := &Server{
		jobOwners:  make(map[string]string),
		mu:         &sync.RWMutex{},
		supervisor: supervisor.NewSupervisor(),
	}

	authInterceptor := NewAuthorizationInterceptor(s)

	s.srv = grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(authInterceptor.unaryServerInterceptor),
		grpc.StreamInterceptor(authInterceptor.streamServerInterceptor),
	)
	api.RegisterJobworkerServiceServer(s.srv, s)

	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	s.l = l

	return s, nil
}

func (s *Server) Serve() error {
	return s.srv.Serve(s.l)
}

func (s *Server) Close() error {
	return s.l.Close()
}

func (s *Server) Start(ctx context.Context, job *api.Job) (*api.JobID, error) {
	commonName, err := authentication.GetCommonNameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if len(job.Command) == 0 {
		return nil, ErrEmptyCommand
	}

	jobID, err := s.supervisor.StartJob(job.Command, job.Arguments...)
	if err != nil {
		return nil, status.Error(codes.Aborted, err.Error())
	}

	s.mu.Lock()
	s.jobOwners[jobID] = commonName
	s.mu.Unlock()

	resp := &api.JobID{Id: string(jobID)}

	return resp, nil
}

func (s *Server) Stop(ctx context.Context, jobID *api.JobID) (*api.StopResponse, error) {
	err := s.supervisor.StopJob(jobID.Id)
	if err == supervisor.ErrJobFinished {
		err = status.Error(codes.FailedPrecondition, err.Error())
	} else if err != nil {
		err = status.Error(codes.Internal, err.Error())
	}

	return &api.StopResponse{}, err
}

func (s *Server) Status(context context.Context, jobID *api.JobID) (*api.StatusResponse, error) {
	st, err := s.supervisor.JobStatus(jobID.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var status api.Status

	switch st.Status {
	case supervisor.StatusStarted:
		status = api.Status_STARTED
	case supervisor.StatusDone:
		status = api.Status_DONE
	case supervisor.StatusStopped:
		status = api.Status_STOPPED
	}

	return &api.StatusResponse{
		Status:   status,
		ExitCode: int64(st.ExitCode),
	}, nil
}

func stream(jobID *api.JobID, srv grpc.ServerStream, sendFn func(*api.OutputChunk) error, fn func(string) (*multipipe.Reader, error)) error {
	out, err := fn(jobID.Id)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	buf := make([]byte, 8192)
	for eof := false; !eof; {
		n, err := out.Read(buf)

		if err == io.EOF {
			eof = true
		} else if err != nil {
			return status.Error(codes.Aborted, err.Error())
		}

		if err := sendFn(&api.OutputChunk{
			Output: buf[:n],
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) StdOut(jobID *api.JobID, srv api.JobworkerService_StdOutServer) error {
	return stream(jobID, srv, srv.Send, s.supervisor.JobStdOut)
}

func (s *Server) StdErr(jobID *api.JobID, srv api.JobworkerService_StdErrServer) error {
	return stream(jobID, srv, srv.Send, s.supervisor.JobStdErr)
}
