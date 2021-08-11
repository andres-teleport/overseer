package authentication

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

func assertNil(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func assertNonNil(t *testing.T, err error) {
	if err == nil {
		t.Fatal("non-nil error expected")
	}
}

func getServerAddress(l net.Listener) string {
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return net.JoinHostPort("localhost", port)
}

func newTestServer(credFn func() (credentials.TransportCredentials, error)) (*grpc.Server, net.Listener, error) {
	creds, err := credFn()
	if err != nil {
		return nil, nil, err
	}

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, nil, err
	}

	return grpc.NewServer(grpc.Creds(creds)), l, nil
}

func knownServerCredentials() (credentials.TransportCredentials, error) {
	return NewServerTransportCredentials(
		"test-assets/server.key",
		"test-assets/server.crt",
		"test-assets/ca.crt",
	)
}

// newUnknownTestServerCredentials returns server credentials with a certificate
// signed by an unknown CA
func unknownServerCredentials() (credentials.TransportCredentials, error) {
	return NewServerTransportCredentials(
		"test-assets/unknown-server.key",
		"test-assets/unknown-server.crt",
		"test-assets/ca.crt",
	)
}

func newTestClient(addr string, credFn func() (credentials.TransportCredentials, error)) (*grpc.ClientConn, error) {
	creds, err := credFn()
	if err != nil {
		return nil, err
	}

	return grpc.Dial(addr, grpc.WithTransportCredentials(creds))
}

func knownUserCredentials() (credentials.TransportCredentials, error) {
	return NewClientTransportCredentials(
		"test-assets/user.key",
		"test-assets/user.crt",
		"test-assets/ca.crt",
	)
}

// unknownUserCredentials returns user credentials with a certificate signed by
// an unknown CA
func unknownUserCredentials() (credentials.TransportCredentials, error) {
	return NewClientTransportCredentials(
		"test-assets/unknown-user.key",
		"test-assets/unknown-user.crt",
		"test-assets/ca.crt",
	)
}

func TestUnknownServerAuthentication(t *testing.T) {
	srv, l, err := newTestServer(unknownServerCredentials)
	assertNil(t, err)

	go srv.Serve(l)
	defer srv.Stop()

	conn, err := newTestClient(getServerAddress(l), knownUserCredentials)
	assertNil(t, err)
	defer conn.Close()

	err = conn.Invoke(context.Background(), "fake", nil, nil)
	assertNonNil(t, err)

	statusCode := status.Convert(err).Code()
	if statusCode != codes.Unavailable {
		t.Fatalf("'%s' expected, '%s' got", codes.Unavailable, statusCode)
	}
}

func TestKnownClientAuthentication(t *testing.T) {
	srv, l, err := newTestServer(knownServerCredentials)
	assertNil(t, err)

	go srv.Serve(l)
	defer srv.Stop()

	conn, err := newTestClient(getServerAddress(l), knownUserCredentials)
	assertNil(t, err)
	defer conn.Close()

	err = conn.Invoke(context.Background(), "fake", nil, nil)
	assertNonNil(t, err)

	statusCode := status.Convert(err).Code()
	if statusCode != codes.Unimplemented {
		t.Fatalf("'%s' expected, '%s' got", codes.Unimplemented, statusCode)
	}
}

// TestUnknownClientAuthentication tests a user certificate signed by an unknown
// CA
func TestUnknownClientAuthentication(t *testing.T) {
	srv, l, err := newTestServer(knownServerCredentials)
	assertNil(t, err)

	go srv.Serve(l)
	defer srv.Stop()

	conn, err := newTestClient(getServerAddress(l), unknownUserCredentials)
	assertNil(t, err)
	defer conn.Close()

	err = conn.Invoke(context.Background(), "fake", nil, nil)
	assertNonNil(t, err)

	statusCode := status.Convert(err).Code()
	if statusCode != codes.Unavailable {
		t.Fatalf("'%s' expected, '%s' got", codes.Unavailable, statusCode)
	}
}

func TestInvalidCreds(t *testing.T) {
	cs := []struct {
		test string
		key  string
		cert string
		ca   string
	}{
		{"invalid key", "authentication_test.go", "test-assets/server.crt", "test-assets/ca.crt"},
		{"invalid key 2", "/", "test-assets/server.crt", "test-assets/ca.crt"},
		{"invalid cert", "test-assets/server.key", "authentication_test.go", "test-assets/ca.crt"},
		{"invalid cert 2", "test-assets/server.key", "/", "test-assets/ca.crt"},
		{"invalid CA", "test-assets/server.key", "test-assets/server.crt", "authentication_test.go"},
		{"invalid CA 2", "test-assets/server.key", "test-assets/server.crt", "/"},
	}

	for _, c := range cs {
		_, _, err := newTestServer(func() (credentials.TransportCredentials, error) {
			return NewServerTransportCredentials(c.key, c.cert, c.ca)
		})
		if err == nil {
			t.Errorf("%s, got nil error", c.test)
		}
	}
}
