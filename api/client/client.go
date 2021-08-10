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

func (c *Client) Start(ctx context.Context, command string, arguments ...string) (string, error) {
	job := &api.Job{
		Command:   command,
		Arguments: arguments,
	}

	jobID, err := c.client.Start(ctx, job)
	if err != nil {
		return "", err
	}

	return jobID.Id, nil
}

func (c *Client) Stop(ctx context.Context, jobID string) error {
	_, err := c.client.Stop(ctx, &api.JobID{Id: jobID})
	return err
}

func (c *Client) Status(ctx context.Context, jobID string) (*api.StatusResponse, error) {
	return c.client.Status(ctx, &api.JobID{Id: jobID})
}

func copyStream(ctx context.Context, streamer api.JobworkerService_StdOutClient, w *io.PipeWriter) {
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

func (c *Client) StdOut(ctx context.Context, jobID string) (*io.PipeReader, error) {
	client, err := c.client.StdOut(ctx, &api.JobID{Id: jobID})
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go copyStream(ctx, client, pw)

	return pr, nil
}

func (c *Client) StdErr(ctx context.Context, jobID string) (*io.PipeReader, error) {
	client, err := c.client.StdErr(ctx, &api.JobID{Id: jobID})

	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go copyStream(ctx, client, pw)

	return pr, nil
}
