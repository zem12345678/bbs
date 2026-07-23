package http

import (
	"net"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestStartReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	server, err := New(&Options{Host: "127.0.0.1", Port: port}, zap.NewNop(), gin.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err == nil {
		t.Fatal("expected occupied port to fail during Start")
	}
}
