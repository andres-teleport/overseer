package server

import (
	"context"

	"github.com/andres-teleport/overseer/api"
	"github.com/andres-teleport/overseer/api/authentication"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrPermissionDenied = status.New(codes.PermissionDenied, "permission denied").Err()
)

type authorizationInterceptor struct {
	parent *Server
}

type authorizationInterceptorServerStream struct {
	authInterceptor *authorizationInterceptor
	grpc.ServerStream
}

func NewAuthorizationInterceptor(s *Server) *authorizationInterceptor {
	return &authorizationInterceptor{
		parent: s,
	}
}

func (a *authorizationInterceptor) userJobAllowed(ctx context.Context, jobID string) error {
	username, err := authentication.GetCommonNameFromCtx(ctx)
	if err != nil {
		return err
	}

	a.parent.mu.RLock()
	defer a.parent.mu.RUnlock()

	if owner, ok := a.parent.jobOwners[jobID]; !ok || owner != username {
		return ErrPermissionDenied
	}

	return nil
}

func (a *authorizationInterceptor) unaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	if r, ok := req.(*api.JobID); ok {
		if err := a.userJobAllowed(ctx, r.Id); err != nil {
			return nil, err
		}
	}

	return handler(ctx, req)
}

func (a *authorizationInterceptor) streamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return handler(srv, &authorizationInterceptorServerStream{a, ss})
}

func (ss *authorizationInterceptorServerStream) RecvMsg(m interface{}) error {
	if err := ss.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	jobID, ok := m.(*api.JobID)
	if !ok {
		return nil
	}

	return ss.authInterceptor.userJobAllowed(ss.Context(), jobID.Id)
}
