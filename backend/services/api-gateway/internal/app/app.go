package app

import (
	stderrors "errors"
	"strings"

	"api-gateway/internal/clients"
	"api-gateway/internal/ioc/application"
	iochttp "api-gateway/internal/ioc/http"
	realtimechat "api-gateway/internal/realtime/chat"

	"github.com/google/wire"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Options struct {
	Name string
}

type Runtime struct {
	realtime *realtimechat.Service
	clients  *clients.Clients
}

func NewRuntime(realtime *realtimechat.Service, clients *clients.Clients) *Runtime {
	return &Runtime{realtime: realtime, clients: clients}
}

func (r *Runtime) Start() error {
	if r == nil || r.realtime == nil {
		return nil
	}
	return r.realtime.Start()
}

func (r *Runtime) Stop() error {
	if r == nil {
		return nil
	}
	var realtimeErr, clientsErr error
	if r.realtime != nil {
		realtimeErr = r.realtime.Stop()
	}
	if r.clients != nil {
		clientsErr = r.clients.Close()
	}
	return stderrors.Join(realtimeErr, clientsErr)
}

func NewOptions(v *viper.Viper, logger *zap.Logger) (*Options, error) {
	o := new(Options)
	if err := v.UnmarshalKey("app", o); err != nil {
		return nil, errors.Wrap(err, "unmarshal app option error")
	}
	if strings.TrimSpace(o.Name) == "" {
		o.Name = firstNonEmpty(v.GetString("service.name"), "bbs-api-gateway")
	}
	logger.Info("load application options success")
	return o, nil
}

func NewApp(o *Options, logger *zap.Logger, hs *iochttp.Server, components ...application.Component) (*application.Application, error) {
	options := []application.Option{application.HttpServerOptions(hs)}
	if len(components) > 0 {
		options = append(options, application.ComponentOptions(components...))
	}
	a, err := application.New(o.Name, logger, options...)
	if err != nil {
		return nil, errors.Wrap(err, "new app error")
	}
	return a, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

var ProviderSet = wire.NewSet(NewApp, NewOptions)
