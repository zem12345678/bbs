package application

import (
	"chat-service/internal/ioc/grpc"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/wire"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Application app server
type Application struct {
	name       string
	logger     *zap.Logger
	grpcServer *grpc.Server
	components []Component
}

// Option app option
type Option func(*Application) error

type Component interface {
	Start() error
	Stop() error
}

// GrpcServerOptions app grpc server option
func GrpcServerOptions(svr *grpc.Server) Option {
	return func(app *Application) error {
		svr.Application(app.name)
		app.grpcServer = svr
		return nil
	}
}

func ComponentOptions(components ...Component) Option {
	return func(app *Application) error {
		app.components = append(app.components, components...)
		return nil
	}
}

// new app
func New(name string, logger *zap.Logger, options ...Option) (*Application, error) {
	app := &Application{
		name:   name,
		logger: logger.With(zap.String("type", "Application")),
	}

	for _, option := range options {
		if err := option(app); err != nil {
			return nil, err
		}
	}

	return app, nil
}

// start app server
func (a *Application) Start() error {
	if a.grpcServer != nil {
		if err := a.grpcServer.Start(); err != nil {
			return errors.Wrap(err, "grpc server start error")
		}
	}
	for _, component := range a.components {
		if err := component.Start(); err != nil {
			return errors.Wrap(err, "application component start error")
		}
	}
	return nil
}

// AwaitSignal await signal for exit app server
func (a *Application) AwaitSignal() {
	c := make(chan os.Signal, 1)
	signal.Reset(syscall.SIGTERM, syscall.SIGINT)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	s := <-c
	a.logger.Info("receive a signal", zap.String("signal", s.String()))
	if a.grpcServer != nil {
		if err := a.grpcServer.Stop(); err != nil {
			a.logger.Error("stop grpc server error", zap.Error(err))
		}
	}
	for _, component := range a.components {
		if err := component.Stop(); err != nil {
			a.logger.Error("stop application component error", zap.Error(err))
		}
	}
	os.Exit(0)
}

// ProviderSet wire 注入
var ProviderSet = wire.NewSet(New)
