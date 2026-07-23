package app

import (
	"strings"

	"chat-service/internal/ioc/application"
	iocgrpc "chat-service/internal/ioc/grpc"

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
		o.Name = StringDefault(v.GetString("grpc.server.serviceName"), StringDefault(v.GetString("service.name"), "bbs-chat-service"))
	}
	logger.Info("load application options success")
	return o, nil
}

func NewApp(o *Options, logger *zap.Logger, server *iocgrpc.Server, runner *RuntimeRunner) (*application.Application, error) {
	options := []application.Option{application.GrpcServerOptions(server)}
	if runner != nil {
		options = append(options, application.ComponentOptions(runner))
	}
	app, err := application.New(o.Name, logger, options...)
	if err != nil {
		return nil, errors.Wrap(err, "new app error")
	}
	return app, nil
}

func StringDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

var ProviderSet = wire.NewSet(NewApp, NewOptions)
