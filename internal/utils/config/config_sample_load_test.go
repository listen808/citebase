package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

// samplePath 指向仓库里那份"新用户照着抄"的模板。
// README 的第一步就是 cp configs/config_sample.yaml configs/config.yaml，
// 所以这份文件解析不了 = 照着文档装的人一个都启动不起来。
const samplePath = "../../../configs/config_sample.yaml"

func loadSample(t *testing.T) *viper.Viper {
	t.Helper()

	v := viper.New()
	v.SetConfigFile(samplePath)
	// yaml.v3 对同一个映射里的重复键报 TypeError，viper 把它当读取失败返回。
	// 模板里曾经同时留着两条 server.trusted_proxies（两份注释互相矛盾），
	// 后果不是"取后一条"而是整份文件读不进来，Load 直接 Fatal。
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("config_sample.yaml 解析失败（重复键或语法错误都会到这里）: %v", err)
	}
	return v
}

func TestConfigSampleIsLoadable(t *testing.T) {
	v := loadSample(t)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("config_sample.yaml 无法解进 Config: %v", err)
	}

	// 模板必须给部署时真要改的项留出可见的默认值，缺项会被静默当成零值。
	if cfg.Server.Port == 0 {
		t.Error("server.port 缺失或为 0：模板必须写出端口默认值")
	}
	if cfg.Server.Host == "" {
		t.Error("server.host 缺失：模板必须写出监听地址默认值")
	}
	if cfg.Database.Driver == "" {
		t.Error("database.driver 缺失：模板必须写出驱动")
	}
}

// TestSampleKeysMatchStructFields 拦"配置项名写错"这一类静默失效：
// viper 对认不出的键不报错，只是不解进结构体，于是用户改了配置而程序按默认值跑。
func TestSampleKeysMatchStructFields(t *testing.T) {
	v := loadSample(t)

	sectionFields := map[string]any{
		"server":   ServerConfig{},
		"database": DatabaseConfig{},
	}

	for section, proto := range sectionFields {
		sub := v.Sub(section)
		if sub == nil {
			t.Errorf("模板缺少 %s 段", section)
			continue
		}
		known := mapstructureKeys(proto)
		for _, key := range sub.AllKeys() {
			if !known[key] {
				t.Errorf("%s.%s 在 %s 里没有对应字段（写错的配置项不会报错，只会被忽略）",
					section, key, reflect.TypeOf(proto).Name())
			}
		}
	}
}

func mapstructureKeys(proto any) map[string]bool {
	out := map[string]bool{}
	rv := reflect.TypeOf(proto)
	for i := 0; i < rv.NumField(); i++ {
		tag := rv.Field(i).Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}
		out[tag] = true
	}
	return out
}
