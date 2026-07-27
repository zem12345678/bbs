package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientTLSOptions configures mutually authenticated TLS for internal gRPC
// calls. Certificate paths are deliberately configuration only; the private
// material must be mounted from a deployment secret rather than stored in
// Nacos or a checked-in config file.
type ClientTLSOptions struct {
	CAFile     string
	CertFile   string
	KeyFile    string
	ServerName string
}

func newClientTransportCredentials(options ClientTLSOptions, secure bool, fallbackServerName string) (credentials.TransportCredentials, error) {
	if !secure {
		return insecure.NewCredentials(), nil
	}

	caFile := strings.TrimSpace(options.CAFile)
	certFile := strings.TrimSpace(options.CertFile)
	keyFile := strings.TrimSpace(options.KeyFile)
	serverName := strings.TrimSpace(options.ServerName)
	if serverName == "" {
		serverName = strings.TrimSpace(fallbackServerName)
	}
	if caFile == "" || certFile == "" || keyFile == "" || serverName == "" {
		return nil, fmt.Errorf("secure grpc client requires tls.caFile, tls.certFile, tls.keyFile, and tls.serverName")
	}

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read grpc client CA file: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse grpc client CA file %q", caFile)
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load grpc client certificate: %w", err)
	}

	return credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      roots,
		Certificates: []tls.Certificate{certificate},
		ServerName:   serverName,
	}), nil
}
