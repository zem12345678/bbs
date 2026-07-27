package grpc

import (
	"testing"

	stdgrpc "google.golang.org/grpc"
)

func TestInitServersRegistersInternalNotificationWriterSeparately(t *testing.T) {
	server := stdgrpc.NewServer()
	NewInitServers(&Handler{})(server)
	services := server.GetServiceInfo()
	if _, ok := services["bbs.notification.v1.NotificationService"]; !ok {
		t.Fatal("user-facing notification service was not registered")
	}
	internal, ok := services["bbs.notification.v1.InternalNotificationService"]
	if !ok {
		t.Fatal("internal notification writer service was not registered")
	}
	if len(internal.Methods) != 1 || internal.Methods[0].Name != "DispatchSystemNotifications" {
		t.Fatalf("internal notification methods = %#v", internal.Methods)
	}
}
