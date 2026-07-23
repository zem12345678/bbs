package application

import (
	iocgrpc "api-gateway/internal/ioc/grpc"
	iochttp "api-gateway/internal/ioc/http"
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
	grpcServer *iocgrpc.Server
	httpServer *iochttp.Server
	components []Component
}

// Option app option
type Option func(*Application) error

type Component interface {
	Start() error
	Stop() error
}

// GrpcServerOptions app grpc server option
func GrpcServerOptions(svr *iocgrpc.Server) Option {
	return func(app *Application) error {
		svr.Application(app.name)
		app.grpcServer = svr
		return nil
	}
}

func HttpServerOptions(svr *iochttp.Server) Option {
	return func(app *Application) error {
		svr.Application(app.name)
		app.httpServer = svr
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
	grpcStarted := false
	if a.grpcServer != nil {
		if err := a.grpcServer.Start(); err != nil {
			a.stopComponents(startedComponents)
			return errors.Wrap(err, "grpc server start error")
		}
		grpcStarted = true
	}
	if a.httpServer != nil {
		if err := a.httpServer.Start(); err != nil {
			if grpcStarted {
				_ = a.grpcServer.Stop()
			}
			a.stopComponents(startedComponents)
			return errors.Wrap(err, "http server start error")
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
	if a.httpServer != nil {
		if err := a.httpServer.Stop(); err != nil {
			a.logger.Error("stop http server error", zap.Error(err))
		}
	}
	if a.grpcServer != nil {
		if err := a.grpcServer.Stop(); err != nil {
			a.logger.Error("stop grpc server error", zap.Error(err))
		}
	}
	a.stopComponents(len(a.components))
	os.Exit(0)
}

func (a *Application) stopComponents(count int) {
	for i := count - 1; i >= 0; i-- {
		if err := a.components[i].Stop(); err != nil {
			a.logger.Error("stop application component error", zap.Error(err))
		}
	}
}

// ProviderSet wire 注入
var ProviderSet = wire.NewSet(New)
