package grpc

import (
	"testing"

	"admin/pkg/logger"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func TestNewServerOptionsReadsInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9114)
	v.Set("grpc.server.serviceName", "bbs-admin-service")
	v.Set("grpc.server.internalAuthToken", " admin-internal-token ")

	o, err := NewServerOptions(v, logger.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.InternalAuthToken != "admin-internal-token" {
		t.Fatalf("internal auth token = %q", o.InternalAuthToken)
	}
}

func TestNewServerOptionsDoesNotLogInternalAuthToken(t *testing.T) {
	v := viper.New()
	v.Set("grpc.server.port", 9114)
	v.Set("grpc.server.serviceName", "bbs-admin-service")
	v.Set("grpc.server.internalAuthToken", "must-not-appear-in-logs")
	log := &capturingLogger{}

	if _, err := NewServerOptions(v, log); err != nil {
		t.Fatal(err)
	}
	for _, field := range log.infoFields {
		if field.Key == "grpc options" || field.Value == "must-not-appear-in-logs" {
			t.Fatalf("internal auth token leaked into log field %#v", field)
		}
	}
}

type capturingLogger struct {
	infoFields []logger.Field
}

func (l *capturingLogger) Debug(string, ...logger.Field) {}
func (l *capturingLogger) Info(_ string, fields ...logger.Field) {
	l.infoFields = append(l.infoFields, fields...)
}
func (l *capturingLogger) Warn(string, ...logger.Field)  {}
func (l *capturingLogger) Error(string, ...logger.Field) {}
func (l *capturingLogger) With(...logger.Field) logger.Logger {
	return l
}
func (l *capturingLogger) GetZapLogger() *zap.Logger { return zap.NewNop() }
