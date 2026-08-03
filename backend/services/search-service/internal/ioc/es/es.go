package es

import (
	"strings"

	elastic "github.com/elastic/go-elasticsearch/v9"
	"github.com/google/wire"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Indices struct {
	Articles          string `yaml:"articles" mapstructure:"articles"`
	Topics            string `yaml:"topics" mapstructure:"topics"`
	Users             string `yaml:"users" mapstructure:"users"`
	AccountTombstones string `yaml:"accountTombstones" mapstructure:"accountTombstones"`
}

type Options struct {
	URL               []string `yaml:"url" mapstructure:"url"`
	EnableDebugLogger bool     `yaml:"enable_debug_logger" mapstructure:"enable_debug_logger"`
	Indices           Indices  `yaml:"indices" mapstructure:"indices"`
}

// NewOptions for ES
func NewOptions(v *viper.Viper, logger *zap.Logger) (*Options, error) {
	var (
		err error
		o   = new(Options)
	)
	if err = v.UnmarshalKey("es", o); err != nil {
		logger.Error("unmarshal es option error", zap.Error(err))
		return nil, err
	}
	if len(o.URL) == 0 {
		o.URL = v.GetStringSlice("elasticsearch.addresses")
	}
	o.URL = normalizeURLs(o.URL)
	if len(o.URL) == 0 {
		o.URL = []string{"http://127.0.0.1:9200"}
	}
	if strings.TrimSpace(o.Indices.Articles) == "" {
		o.Indices.Articles = strings.TrimSpace(v.GetString("elasticsearch.indices.articles"))
	}
	if strings.TrimSpace(o.Indices.Articles) == "" {
		o.Indices.Articles = "bbs_articles"
	}
	if strings.TrimSpace(o.Indices.Topics) == "" {
		o.Indices.Topics = strings.TrimSpace(v.GetString("elasticsearch.indices.topics"))
	}
	if strings.TrimSpace(o.Indices.Topics) == "" {
		o.Indices.Topics = "bbs_topics"
	}
	if strings.TrimSpace(o.Indices.Users) == "" {
		o.Indices.Users = strings.TrimSpace(v.GetString("elasticsearch.indices.users"))
	}
	if strings.TrimSpace(o.Indices.Users) == "" {
		o.Indices.Users = "bbs_users_v2"
	}
	if strings.TrimSpace(o.Indices.AccountTombstones) == "" {
		o.Indices.AccountTombstones = strings.TrimSpace(v.GetString("elasticsearch.indices.accountTombstones"))
	}
	if strings.TrimSpace(o.Indices.AccountTombstones) == "" {
		o.Indices.AccountTombstones = "bbs_search_account_tombstones_v1"
	}

	logger.Info("load es options success", zap.Any("es options", o))
	return o, nil
}

// New 初始化ES连接信息
func New(o *Options) (*elastic.Client, error) {
	cfg := elastic.Config{
		Addresses:         o.URL,
		EnableDebugLogger: o.EnableDebugLogger,
	}

	client, err := elastic.NewClient(cfg)

	if err != nil {
		return nil, err
	}
	return client, nil
}

func normalizeURLs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// ProviderSet inject es settings
var ProviderSet = wire.NewSet(New, NewOptions)
