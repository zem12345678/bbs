package grpc

import (
	"testing"

	"chat-service/pkg/logger"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

func TestNewServerOptionsReadsBindAndAdvertiseHosts(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.host", "0.0.0.0")
	v.Set("grpc.server.advertiseHost", "10.0.0.8")
	v.Set("grpc.server.port", 9116)
	v.Set("grpc.server.serviceName", "bbs-chat-service")
	v.Set("grpc.server.internalAuthToken", "test-internal-token")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.Host != "0.0.0.0" || o.AdvertiseHost != "10.0.0.8" {
		t.Fatalf("hosts = %q, %q", o.Host, o.AdvertiseHost)
	}
	if o.InternalAuthToken != "test-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}

func TestResolveServerAddressesUsesConfiguredBindAndAdvertiseHosts(t *testing.T) {
	listenAddr, advertiseAddr, err := resolveServerAddresses("0.0.0.0", "10.0.0.8", "192.168.1.8", 9116)
	if err != nil {
		t.Fatal(err)
	}
	if listenAddr != "0.0.0.0:9116" {
		t.Fatalf("listen address = %q", listenAddr)
	}
	if advertiseAddr != "10.0.0.8:9116" {
		t.Fatalf("advertise address = %q", advertiseAddr)
	}
}

func TestResolveServerAddressesFallsBackToLocalIP(t *testing.T) {
	listenAddr, advertiseAddr, err := resolveServerAddresses("", "", "192.168.1.8", 9116)
	if err != nil {
		t.Fatal(err)
	}
	if listenAddr != "192.168.1.8:9116" || advertiseAddr != "192.168.1.8:9116" {
		t.Fatalf("addresses = %q, %q", listenAddr, advertiseAddr)
	}
}

func TestResolveServerAddressesUsesFallbackOnlyForMissingHost(t *testing.T) {
	listenAddr, advertiseAddr, err := resolveServerAddresses("", "10.0.0.8", "192.168.1.8", 9116)
	if err != nil {
		t.Fatal(err)
	}
	if listenAddr != "192.168.1.8:9116" || advertiseAddr != "10.0.0.8:9116" {
		t.Fatalf("addresses = %q, %q", listenAddr, advertiseAddr)
	}
}

func TestResolveServerAddressesSupportsIPv6(t *testing.T) {
	listenAddr, advertiseAddr, err := resolveServerAddresses("::", "2001:db8::8", "", 9116)
	if err != nil {
		t.Fatal(err)
	}
	if listenAddr != "[::]:9116" || advertiseAddr != "[2001:db8::8]:9116" {
		t.Fatalf("addresses = %q, %q", listenAddr, advertiseAddr)
	}
}

func TestResolveServerAddressesRequiresFallbackForMissingAddress(t *testing.T) {
	if _, _, err := resolveServerAddresses("0.0.0.0", "", "", 9116); err == nil {
		t.Fatal("resolveServerAddresses() error = nil")
	}
}

func TestStopUnregistersService(t *testing.T) {
	registration := &registrationStub{}
	server := &Server{
		o:            &ServerOptions{ServiceName: "bbs-chat-service"},
		logger:       logger.NewNopLogger(),
		server:       grpc.NewServer(),
		registration: registration,
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if registration.stops != 1 {
		t.Fatalf("registration stops = %d, want 1", registration.stops)
	}
	if server.registration != nil {
		t.Fatal("registration was not cleared")
	}
}

func TestDialRejectsUnsupportedSecureConnection(t *testing.T) {
	client := &Client{o: &ClientOptions{}}

	if _, err := client.Dial("bbs-chat-service", true); err == nil {
		t.Fatal("Dial() error = nil")
	}
}

type registrationStub struct {
	stops int
}

func (s *registrationStub) Stop() {
	s.stops++
}
