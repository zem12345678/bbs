package app

import (
	"strings"

	"api-gateway/internal/ioc/application"
	iochttp "api-gateway/internal/ioc/http"

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
		o.Name = firstNonEmpty(v.GetString("service.name"), "api-gateway")
	}
	logger.Info("load application options success")
	return o, nil
}

func NewApp(o *Options, logger *zap.Logger, hs *iochttp.Server) (*application.Application, error) {
	a, err := application.New(o.Name, logger, application.HttpServerOptions(hs))
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
