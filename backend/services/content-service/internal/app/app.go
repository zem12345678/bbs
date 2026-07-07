package app

import (
	"strings"

	"content-service/internal/ioc/application"
	"content-service/internal/ioc/grpc"

	"github.com/google/wire"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Options struct {
	Name string
}

func NewOptions(v *viper.Viper, logger *zap.Logger) (*Options, error) {
	o := new(Options)
	if err := v.UnmarshalKey("app", o); err != nil {
		return nil, errors.Wrap(err, "unmarshal app option error")
	}
	if strings.TrimSpace(o.Name) == "" {
		o.Name = StringDefault(v.GetString("grpc.server.serviceName"), StringDefault(v.GetString("service.name"), "content-service"))
	}
	logger.Info("load application options success")
	return o, nil
}

func NewApp(o *Options, logger *zap.Logger, gs *grpc.Server) (*application.Application, error) {
	a, err := application.New(o.Name, logger, application.GrpcServerOptions(gs))
	if err != nil {
		return nil, errors.Wrap(err, "new app error")
	}
	return a, nil
}

func StringDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

var ProviderSet = wire.NewSet(NewApp, NewOptions)
