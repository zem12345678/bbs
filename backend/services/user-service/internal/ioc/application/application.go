package application

import (
	"os"
	"os/signal"
	"syscall"
	"user-service/internal/ioc/grpc"

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
	startedComponents := 0
	for _, component := range a.components {
		if err := component.Start(); err != nil {
			a.stopComponents(startedComponents)
			return errors.Wrap(err, "application component start error")
		}
		startedComponents++
	}
	if a.grpcServer != nil {
		if err := a.grpcServer.Start(); err != nil {
			a.stopComponents(startedComponents)
			return errors.Wrap(err, "grpc server start error")
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
	a.stopComponents(len(a.components))
	os.Exit(0)
}

func (a *Application) stopComponents(count int) {
	for index := count - 1; index >= 0; index-- {
		if err := a.components[index].Stop(); err != nil {
			a.logger.Error("stop application component error", zap.Error(err))
		}
	}
}

// ProviderSet wire 注入
var ProviderSet = wire.NewSet(New)
