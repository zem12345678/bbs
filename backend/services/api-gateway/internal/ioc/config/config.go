package config

import (
	"api-gateway/pkg/uuid"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Options struct {
	Addr        string `mapstructure:"addr" toml:"addr" json:"addr" yaml:"addr" env:"NACOS_ADDR"`
	Port        uint64 `mapstructure:"port" toml:"port" json:"port" yaml:"port" env:"NACOS_PORT"`
	NamespaceID string `mapstructure:"namespaceId" toml:"namespaceId" json:"namespaceId" yaml:"namespaceId" env:"NACOS_NAMESPACEID"`
	DataID      string `mapstructure:"dataId" toml:"dataId" json:"dataId" yaml:"dataId" env:"NACOS_DATAID"`
	GroupID     string `mapstructure:"groupId" toml:"groupId" json:"groupId" yaml:"groupId" env:"NACOS_GROUPID"`
}

func New(path string) (*viper.Viper, error) {
	var (
		err error
		v   = viper.New()
		o   = new(Options)
	)
	v.AddConfigPath(".")
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err == nil {
		fmt.Printf("use config file -> %s\n", v.ConfigFileUsed())
	} else {
		return nil, errors.Wrap(err, "read config file error")
	}
	if !skipNacos() {
		// Nacos is the bootstrap configuration source, so its environment
		// overrides must be applied before creating the Nacos client.
		if err = applyNacosEnvOverrides(v); err != nil {
			return nil, err
		}
		if err = v.UnmarshalKey("nacos", o); err != nil {
			return nil, errors.Wrap(err, "unmarshal nacos option error")
		}
		group := stringDefault(o.GroupID, "DEFAULT_GROUP")

		sc := []constant.ServerConfig{
			{
				IpAddr: o.Addr,
				Port:   o.Port,
			},
		}
		cc := constant.ClientConfig{
			NamespaceId:         o.NamespaceID,
			TimeoutMs:           5000,
			NotLoadCacheAtStart: true,
			LogDir:              "tmp/nacos/log",
			CacheDir:            "tmp/nacos/cache",
			LogLevel:            "debug",
		}

		configClient, err := clients.CreateConfigClient(map[string]interface{}{
			"serverConfigs": sc,
			"clientConfig":  cc,
		})
		if err != nil {
			return nil, err
		}
		content, err := configClient.GetConfig(vo.ConfigParam{
			DataId: o.DataID,
			Group:  group})
		if err != nil {
			return nil, err
		}
		if err = v.MergeConfig(bytes.NewBufferString(content)); err != nil {
			return nil, errors.Wrap(err, "viper read nacos config error")
		}

		err = configClient.ListenConfig(vo.ConfigParam{
			DataId: o.DataID,
			Group:  group,
			OnChange: func(namespace, group, dataID, data string) {
				_ = namespace
				_ = group
				_ = dataID
				_ = v.MergeConfig(bytes.NewBufferString(data))
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "listenConfig nacos config error")
		}
	}
	uuidstr, err := uuid.GetHostUuid()
	if err != nil || uuidstr == "" {
		fmt.Println("new uuid")
		uuidstr, err = uuid.NewUUID()
	}
	setNestedConfigValue(v, "server.uuid", uuidstr)
	return v, err
}

// skipNacos lets an immutable deployment configuration be the sole startup
// source. It is deliberately an environment-only switch so a mutable Nacos
// payload cannot turn itself off or on after the process starts.
func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_GATEWAY_SKIP_NACOS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func applyNacosEnvOverrides(v *viper.Viper) error {
	setStringFromEnv(v, "nacos.addr", "BBS_GATEWAY_NACOS_ADDR")
	setStringFromEnv(v, "nacos.namespaceId", "BBS_GATEWAY_NACOS_NAMESPACE_ID", "BBS_GATEWAY_NACOS_NAMESPACEID")
	setStringFromEnv(v, "nacos.dataId", "BBS_GATEWAY_NACOS_DATA_ID", "BBS_GATEWAY_NACOS_DATAID")
	setStringFromEnv(v, "nacos.groupId", "BBS_GATEWAY_NACOS_GROUP_ID", "BBS_GATEWAY_NACOS_GROUPID")
	if value := firstEnv("BBS_GATEWAY_NACOS_PORT"); value != "" {
		port, err := strconv.ParseUint(value, 10, 16)
		if err != nil || port == 0 {
			return fmt.Errorf("invalid BBS_GATEWAY_NACOS_PORT %q", value)
		}
		setNestedConfigValue(v, "nacos.port", port)
	}
	return nil
}

func setStringFromEnv(v *viper.Viper, key string, envs ...string) {
	if value := firstEnv(envs...); value != "" {
		setNestedConfigValue(v, key, value)
	}
}

func firstEnv(envs ...string) string {
	for _, env := range envs {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			return value
		}
	}
	return ""
}

func stringDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

var ProviderSet = wire.NewSet(New)

// setNestedConfigValue writes value at a dotted key without dropping sibling keys.
//
// viper's Set publishes the value in the override layer, and that layer stores it as a
// partial nested map. A whole-subtree read such as UnmarshalKey("grpc.server", &o) finds
// the override subtree first and returns only the keys present there, silently discarding
// siblings that came from the config file, so writing a single leaf through Set would break
// unrelated settings. MergeConfigMap keeps siblings but writes to the config layer, which
// AutomaticEnv/BindEnv outrank, so a CSV list value would lose to the raw env string.
//
// Snapshot the whole top-level subtree through AllKeys/Get so every sibling keeps its fully
// resolved value (including env-provided ones), apply the new leaf, then republish the entire
// root in the override layer. Siblings survive and the write still wins over env bindings.
func setNestedConfigValue(v *viper.Viper, key string, value interface{}) {
	parts := strings.Split(strings.ToLower(key), ".")
	if len(parts) == 1 {
		v.Set(parts[0], value)
		return
	}
	root := parts[0]
	prefix := root + "."

	tree := map[string]interface{}{}
	for _, full := range v.AllKeys() {
		if !strings.HasPrefix(full, prefix) {
			continue
		}
		assignNestedConfigValue(tree, strings.Split(strings.TrimPrefix(full, prefix), "."), v.Get(full))
	}
	assignNestedConfigValue(tree, parts[1:], value)
	v.Set(root, tree)
}

// assignNestedConfigValue writes value into tree at path, creating intermediate maps.
func assignNestedConfigValue(tree map[string]interface{}, path []string, value interface{}) {
	node := tree
	for _, segment := range path[:len(path)-1] {
		next, ok := node[segment].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			node[segment] = next
		}
		node = next
	}
	node[path[len(path)-1]] = value
}
