package server

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"

	"github.com/andres-teleport/overseer/api"
	"github.com/andres-teleport/overseer/api/client"
	"github.com/andres-teleport/overseer/lib/supervisor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func assertNil(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func assertStatusCode(t *testing.T, err error, expected codes.Code) {
	statusCode := status.Convert(err).Code()
	if statusCode != expected {
		t.Errorf("'%s' expected, '%s' got", expected, statusCode)
	}
}

func newTestServer() (*Server, error) {
	return NewServer(
		"localhost:0",
		"test-assets/server.key",
		"test-assets/server.crt",
		"test-assets/ca.crt",
	)
}

func newKnownClient(addr string) (*client.Client, error) {
	return client.NewClient(
		addr,
		"test-assets/user.key",
		"test-assets/user.crt",
		"test-assets/ca.crt",
	)
}

func newAnotherKnownClient(addr string) (*client.Client, error) {
	return client.NewClient(
		addr,
		"test-assets/another-user.key",
		"test-assets/another-user.crt",
		"test-assets/ca.crt",
	)
}

func getServerAddress(l net.Listener) string {
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return net.JoinHostPort("localhost", port)
}

func TestActions(t *testing.T) {
	srv, err := newTestServer()
	assertNil(t, err)

	go srv.Serve()
	defer srv.Close()

	cli, err := newKnownClient(getServerAddress(srv.l))
	assertNil(t, err)

	// Start
	testPhrase := "hello server"
	jobID, err := cli.Start(context.Background(), "echo", testPhrase)
	assertNil(t, err)

	// StdOut
	rd, err := cli.StdOut(context.Background(), jobID)
	assertNil(t, err)

	out, err := io.ReadAll(rd)
	assertNil(t, err)

	out = bytes.TrimSpace(out)
	if !bytes.Equal(out, []byte(testPhrase)) {
		t.Errorf("'%s' expected, '%s' got", testPhrase, out)
	}

	// StdErr
	rd, err = cli.StdErr(context.Background(), jobID)
	assertNil(t, err)

	out, err = io.ReadAll(rd)
	assertNil(t, err)

	out = bytes.TrimSpace(out)
	if len(out) > 0 {
		t.Errorf("'' expected, '%s' got", out)
	}

	// Status
	expectedStatus := api.Status_DONE
	jobStatus, err := cli.Status(context.Background(), jobID)
	assertNil(t, err)

	if jobStatus.Status != expectedStatus {
		t.Errorf("'%s' expected, '%s' got", expectedStatus, jobStatus.Status)
	}

	// Stop
	err = cli.Stop(context.Background(), jobID)
	st := status.Convert(err)
	if st.Message() != supervisor.ErrJobFinished.Error() {
		t.Errorf("'%s' expected, '%s' got", supervisor.ErrJobFinished, err)
	}
}

func TestBadActions(t *testing.T) {
	srv, err := newTestServer()
	assertNil(t, err)

	go srv.Serve()
	defer srv.Close()

	cli, err := newKnownClient(getServerAddress(srv.l))
	assertNil(t, err)

	invalidJobID := "invalid-id"

	// Start
	_, err = cli.Start(context.Background(), "")
	assertStatusCode(t, err, codes.InvalidArgument)

	// StdOut
	rd, err := cli.StdOut(context.Background(), invalidJobID)
	assertNil(t, err)

	_, err = io.ReadAll(rd)
	assertStatusCode(t, err, codes.PermissionDenied)

	rd, err = cli.StdOut(context.Background(), "")
	assertNil(t, err)

	_, err = io.ReadAll(rd)
	assertStatusCode(t, err, codes.PermissionDenied)

	// StdErr
	rd, err = cli.StdErr(context.Background(), invalidJobID)
	assertNil(t, err)

	_, err = io.ReadAll(rd)
	assertStatusCode(t, err, codes.PermissionDenied)

	rd, err = cli.StdErr(context.Background(), "")
	assertNil(t, err)

	_, err = io.ReadAll(rd)
	assertStatusCode(t, err, codes.PermissionDenied)

	// Status
	_, err = cli.Status(context.Background(), invalidJobID)
	assertStatusCode(t, err, codes.PermissionDenied)

	_, err = cli.Status(context.Background(), "")
	assertStatusCode(t, err, codes.PermissionDenied)

	// Stop
	err = cli.Stop(context.Background(), invalidJobID)
	assertStatusCode(t, err, codes.PermissionDenied)

	err = cli.Stop(context.Background(), "")
	assertStatusCode(t, err, codes.PermissionDenied)
}

func TestAuthorization(t *testing.T) {
	srv, err := newTestServer()
	assertNil(t, err)

	go srv.Serve()
	defer srv.Close()

	// User A
	cli, err := newKnownClient(getServerAddress(srv.l))
	assertNil(t, err)

	// User B
	anotherCli, err := newAnotherKnownClient(getServerAddress(srv.l))
	assertNil(t, err)

	// Start (User A)
	testPhrase := "hello server"
	jobID, err := cli.Start(context.Background(), "echo", testPhrase)
	assertNil(t, err)

	// Status (User B)
	_, err = anotherCli.Status(context.Background(), jobID)
	assertStatusCode(t, err, codes.PermissionDenied)

	// Status (User A)
	_, err = cli.Status(context.Background(), jobID)
	assertNil(t, err)

	// StdOut (User B)
	rd, err := anotherCli.StdOut(context.Background(), jobID)
	assertNil(t, err)

	_, err = io.ReadAll(rd)
	assertStatusCode(t, err, codes.PermissionDenied)

	// StdOut (User A)
	rd, err = cli.StdOut(context.Background(), jobID)
	assertNil(t, err)

	out, err := io.ReadAll(rd)
	assertNil(t, err)

	out = bytes.TrimSpace(out)
	if !bytes.Equal(out, []byte(testPhrase)) {
		t.Errorf("'%s' expected, '%s' got", testPhrase, out)
	}
}

func TestGetCommonNameFromCtx(t *testing.T) {
	srv, err := newTestServer()
	assertNil(t, err)

	go srv.Serve()
	defer srv.Close()

	srv.mu.Lock()
	if len(srv.jobOwners) > 0 {
		srv.mu.Unlock()
		t.FailNow()
	}
	srv.mu.Unlock()

	var us = []struct {
		expectedCN  string
		newClientFn func(addr string) (*client.Client, error)
	}{
		{"user", newKnownClient},
		{"another-user", newAnotherKnownClient},
	}

	for _, u := range us {
		cli, err := u.newClientFn(getServerAddress(srv.l))
		assertNil(t, err)

		jobID, err := cli.Start(context.Background(), "echo", "hello")
		assertNil(t, err)

		srv.mu.Lock()
		if owner, ok := srv.jobOwners[jobID]; !ok {
			t.Errorf("'%s' owner expected, got none", owner)
		} else if owner != u.expectedCN {
			t.Errorf("'%s' expected, '%s' got", u.expectedCN, owner)
		}
		srv.mu.Unlock()
	}
}
