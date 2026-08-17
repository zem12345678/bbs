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
	methods := make(map[string]struct{}, len(internal.Methods))
	for _, method := range internal.Methods {
		methods[method.Name] = struct{}{}
	}
	if len(methods) != 3 {
		t.Fatalf("internal notification methods = %#v", internal.Methods)
	}
	for _, name := range []string{"DispatchSystemNotifications", "CreateExportCompletedNotification", "EraseUserData"} {
		if _, ok := methods[name]; !ok {
			t.Fatalf("internal notification methods = %#v", internal.Methods)
		}
	}
	if len(internal.Methods) != len(methods) {
		t.Fatalf("internal notification methods = %#v", internal.Methods)
	}
}
