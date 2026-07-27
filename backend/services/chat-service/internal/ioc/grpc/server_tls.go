package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
)

// ServerTLSOptions configures mutually authenticated TLS for the internal
// gRPC listener. Files must be provided by a deployment secret mount; no
// private key material belongs in Nacos or checked-in configuration.
type ServerTLSOptions struct {
	Enabled      bool
	CertFile     string
	KeyFile      string
	ClientCAFile string
}

func newServerTLSConfig(options ServerTLSOptions) (*tls.Config, error) {
	certFile := strings.TrimSpace(options.CertFile)
	keyFile := strings.TrimSpace(options.KeyFile)
	clientCAFile := strings.TrimSpace(options.ClientCAFile)
	if certFile == "" || keyFile == "" || clientCAFile == "" {
		return nil, fmt.Errorf("grpc server TLS requires tls.certFile, tls.keyFile, and tls.clientCAFile")
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load grpc server certificate: %w", err)
	}
	clientCAPEM, err := os.ReadFile(clientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read grpc server client CA file: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, fmt.Errorf("parse grpc server client CA file %q", clientCAFile)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}, nil
}
