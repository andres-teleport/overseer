package authentication

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var (
	ErrParsingCACert = status.Error(codes.Unauthenticated, "could not parse CA certificate")
	ErrMissingCN     = status.Error(codes.Unauthenticated, "could not get the Common Name from the certificate")
)

func getCerts(keyFile, certFile, caFile string) (cert tls.Certificate, certPool *x509.CertPool, err error) {
	cert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return
	}

	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return
	}

	certPool = x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		err = ErrParsingCACert
		return
	}

	return
}

func NewServerTransportCredentials(keyFile, certFile, caFile string) (credentials.TransportCredentials, error) {
	serverCert, clientCAs, err := getCerts(keyFile, certFile, caFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
	}), nil
}

func NewClientTransportCredentials(keyFile, certFile, caFile string) (credentials.TransportCredentials, error) {
	clientCert, rootCAs, err := getCerts(keyFile, certFile, caFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      rootCAs,
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
	}), nil
}

func GetCommonNameFromCtx(ctx context.Context) (string, error) {
	if p, ok := peer.FromContext(ctx); ok {
		tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
		if !ok ||
			len(tlsInfo.State.VerifiedChains) == 0 ||
			len(tlsInfo.State.VerifiedChains[0]) == 0 ||
			len(tlsInfo.State.VerifiedChains[0][0].Subject.Names) == 0 {

			return "", ErrMissingCN
		}

		if v, ok := tlsInfo.State.VerifiedChains[0][0].Subject.Names[0].Value.(string); ok {
			return v, nil
		}
	}

	return "", ErrMissingCN
}
