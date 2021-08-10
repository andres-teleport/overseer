package client

import (
	"context"
	"io"

	"github.com/andres-teleport/overseer/api"
	"github.com/andres-teleport/overseer/api/authentication"
	"google.golang.org/grpc"
)

type Client struct {
	client api.JobworkerServiceClient
}

type Status struct {
	Status   string
	ExitCode int64
}

func NewClient(serverAddr, keyFile, certFile, caFile string) (*Client, error) {
	creds, err := authentication.NewClientTransportCredentials(keyFile, certFile, caFile)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}

	return &Client{client: api.NewJobworkerServiceClient(conn)}, nil
}

func (c *Client) Start(command string, arguments ...string) (string, error) {
	job := &api.Job{
		Command:   command,
		Arguments: arguments,
	}

	jobID, err := c.client.Start(context.Background(), job)
	if err != nil {
		return "", err
	}

	return jobID.Id, nil
}

func (c *Client) Stop(jobID string) error {
	_, err := c.client.Stop(context.Background(), &api.JobID{Id: jobID})
	return err
}

func (c *Client) Status(jobID string) (Status, error) {
	statusResp, err := c.client.Status(context.Background(), &api.JobID{Id: jobID})
	if err != nil {
		return Status{}, err
	}

	return Status{
		Status:   statusResp.Status.String(),
		ExitCode: statusResp.ExitCode,
	}, nil
}

func copyStream(streamer api.JobworkerService_StdOutClient, w *io.PipeWriter) {
	for eof := false; !eof; {
		chunk, err := streamer.Recv()
		if err == io.EOF {
			eof = true
		} else if err != nil {
			_ = w.CloseWithError(err)
		}

		if chunk != nil {
			if _, err := w.Write(chunk.Output); err != nil {
				_ = w.CloseWithError(err)
				return
			}
		}
	}

	_ = w.Close()
}

func (c *Client) StdOut(jobID string) (*io.PipeReader, error) {
	client, err := c.client.StdOut(context.Background(), &api.JobID{Id: jobID})
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go copyStream(client, pw)

	return pr, nil
}

func (c *Client) StdErr(jobID string) (*io.PipeReader, error) {
	client, err := c.client.StdErr(context.Background(), &api.JobID{Id: jobID})

	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go copyStream(client, pw)

	return pr, nil
}
