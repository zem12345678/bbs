package grpc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc/credentials"
)

func TestClientTransportCredentialsPerformMTLSAndVerifyFallbackServiceName(t *testing.T) {
	caCert, caKey, caPEM := newTestCertificateAuthority(t)
	serverCert, _, _ := newTestLeafCertificate(t, caCert, caKey, 2, []string{"bbs-admin-service"}, x509.ExtKeyUsageServerAuth)
	_, clientCertPEM, clientKeyPEM := newTestLeafCertificate(t, caCert, caKey, 3, nil, x509.ExtKeyUsageClientAuth)

	tlsDir := t.TempDir()
	caFile := writeTestPEM(t, tlsDir, "ca.crt", caPEM)
	certFile := writeTestPEM(t, tlsDir, "tls.crt", clientCertPEM)
	keyFile := writeTestPEM(t, tlsDir, "tls.key", clientKeyPEM)

	clientCredentials, err := newClientTransportCredentials(ClientTLSOptions{
		CAFile:   caFile,
		CertFile: certFile,
		KeyFile:  keyFile,
	}, true, "bbs-admin-service")
	if err != nil {
		t.Fatalf("new client transport credentials: %v", err)
	}

	serverTLS := newTestMTLSServerConfig(t, serverCert, caCert)
	clientErr, serverErr := handshakeWithTestServer(t, serverTLS, clientCredentials)
	if clientErr != nil || serverErr != nil {
		t.Fatalf("mTLS handshake client=%v server=%v", clientErr, serverErr)
	}

	wrongNameCredentials, err := newClientTransportCredentials(ClientTLSOptions{
		CAFile:   caFile,
		CertFile: certFile,
		KeyFile:  keyFile,
	}, true, "bbs-chat-service")
	if err != nil {
		t.Fatalf("new client credentials with wrong fallback: %v", err)
	}
	clientErr, _ = handshakeWithTestServer(t, serverTLS, wrongNameCredentials)
	if clientErr == nil {
		t.Fatal("expected certificate verification to reject the wrong fallback service name")
	}
}

func newTestCertificateAuthority(t *testing.T) (*x509.Certificate, crypto.Signer, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "bbs-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}
	return certificate, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func newTestLeafCertificate(t *testing.T, ca *x509.Certificate, caKey crypto.Signer, serial int64, dnsNames []string, usage x509.ExtKeyUsage) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "bbs-test-leaf"},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatalf("load leaf key pair: %v", err)
	}
	return certificate, certificatePEM, keyPEM
}

func newTestMTLSServerConfig(t *testing.T, certificate tls.Certificate, clientCA *x509.Certificate) *tls.Config {
	t.Helper()
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(clientCA)
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		NextProtos:   []string{"h2"},
	}
}

func writeTestPEM(t *testing.T, directory, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func handshakeWithTestServer(t *testing.T, serverTLS *tls.Config, clientCredentials credentials.TransportCredentials) (error, error) {
	t.Helper()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	serverResult := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			err = connection.(*tls.Conn).Handshake()
			_ = connection.Close()
		}
		serverResult <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rawConnection, err := (&net.Dialer{}).DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial test server: %v", err)
	}
	// grpc-go uses the transport credential's configured ServerName as the
	// connection authority when no explicit authority is supplied. Passing it
	// here exercises the same per-upstream fallback used by the gateway dialer.
	secureConnection, _, clientErr := clientCredentials.ClientHandshake(ctx, clientCredentials.Info().ServerName, rawConnection)
	if secureConnection != nil {
		_ = secureConnection.Close()
	} else {
		_ = rawConnection.Close()
	}

	select {
	case serverErr := <-serverResult:
		return clientErr, serverErr
	case <-ctx.Done():
		t.Fatal("timed out waiting for TLS server handshake")
		return nil, nil
	}
}
