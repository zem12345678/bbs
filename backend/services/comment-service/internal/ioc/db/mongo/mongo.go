package mongo

import (
	"comment-service/pkg/logger"
	"context"
	"strings"
	"time"

	"github.com/google/wire"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"
)

type Options struct {
	URI         string   `toml:"uri" json:"uri" yaml:"uri" mapstructure:"uri" env:"MONGO_URI"`
	UserName    string   `toml:"username" json:"username" yaml:"username" mapstructure:"username" env:"MONGO_USERNAME"`
	Password    string   `toml:"password" json:"password" yaml:"password" mapstructure:"password" env:"MONGO_PASSWORD"`
	Endpoints   []string `toml:"endpoints" json:"endpoints" yaml:"endpoints" mapstructure:"endpoints" env:"MONGO_ENDPOINTS" envSeparator:","`
	AuthDB      string   `toml:"authDB" json:"authDB" yaml:"authDB" mapstructure:"authDB" env:"MONGO_AUTH_DB"`
	EnableTrace bool     `toml:"enableTrace" json:"enableTrace" yaml:"enableTrace" mapstructure:"enableTrace" env:"MONGO_ENABLE_TRACE"`
	Database    string   `toml:"database" json:"database" yaml:"database" mapstructure:"database" env:"MONGO_DATABASE"`
	DB          *mongo.Database
	Client      *mongo.Client
}

type MongoDB struct {
	DB     *mongo.Database
	Client *mongo.Client
}

func NewOptions(v *viper.Viper, l logger.Logger) (*Options, error) {
	var (
		err error
		o   = new(Options)
	)
	if err = v.UnmarshalKey("mongo", o); err != nil {
		return nil, errors.Wrap(err, "unmarshal database mongo option error")
	}

	l.Info("load database options success", logger.Any("database mongo options", o))
	return o, err
}

func New(o *Options) (*MongoDB, error) {
	if !hasConnectionTarget(o) {
		return nil, errors.New("缺少mongo配置")
	} else {
		mongodb, err := initDB(o)
		if err != nil {
			return nil, err
		}
		o.DB = mongodb.Database(o.Database)

	}
	return &MongoDB{
		o.DB,
		o.Client,
	}, nil
}

func initDB(m *Options) (*mongo.Client, error) {
	opts := options.Client()
	if strings.TrimSpace(m.URI) != "" {
		opts.ApplyURI(m.URI)
	} else if m.UserName != "" && m.Password != "" {
		cred := options.Credential{
			AuthSource: m.GetAuthDB(),
		}

		cred.Username = m.UserName
		cred.Password = m.Password
		cred.PasswordSet = true
		opts.SetAuth(cred)
		if endpoints := cleanEndpoints(m.Endpoints); len(endpoints) > 0 {
			opts.SetHosts(endpoints)
		}
	} else if endpoints := cleanEndpoints(m.Endpoints); len(endpoints) > 0 {
		opts.SetHosts(endpoints)
	}
	opts.SetConnectTimeout(5 * time.Second)
	if m.EnableTrace {
		opts.Monitor = otelmongo.NewMonitor(
			otelmongo.WithCommandAttributeDisabled(true),
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	// Connect to MongoDB
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, errors.Wrap(err, "mongo driver open mongodb connection error")
	}
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, errors.Wrap(err, "mongo ping fail")
	}
	//db := client.Database(m.Database)
	m.Client = client
	return client, nil
}

func hasConnectionTarget(o *Options) bool {
	if o == nil {
		return false
	}
	if strings.TrimSpace(o.URI) != "" {
		return true
	}
	return len(cleanEndpoints(o.Endpoints)) > 0
}

func cleanEndpoints(endpoints []string) []string {
	result := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint = strings.TrimSpace(endpoint); endpoint != "" {
			result = append(result, endpoint)
		}
	}
	return result
}

func (m *Options) GetAuthDB() string {
	if m.AuthDB != "" {
		return m.AuthDB
	}

	return m.Database
}
func (db *Options) Close(ctx context.Context) error {
	if db.Client == nil {
		return nil
	}
	return db.Client.Disconnect(ctx)
}

// ProviderSet dependency injection
var ProviderSet = wire.NewSet(New, NewOptions)
