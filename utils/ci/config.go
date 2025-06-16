package ci

import (
	"log"
	"strings"
	"sync"

	"gopkg.in/ini.v1"
)

var (
	once       sync.Once
	instance   *Config
	configPath string
)

type Config struct {
	AppName   string
	LogLevel  string
	AdminPath string
	Mysql     MysqlConfig
	Redis     RedisConfig
	Tenant    struct {
		auth string
	}
	Whitelist []string // 新增字段，用于存储whitelist内容
}

type MysqlConfig struct {
	IP       string
	Port     string
	User     string
	Password string
	Database string
}

type RedisConfig struct {
	IP   string
	Port string
}

func C(key string) string {
	configPath = GetConfigPath("config.ini")
	once.Do(func() {
		instance = &Config{}
		// 使用计算得到的configPath而不是硬编码路径
		cfg, err := ini.Load(configPath)
		if err != nil {
			log.Fatalf("Fail to read file: %v", err)
		}
		// 其他配置加载代码保持不变
		instance.AppName = cfg.Section("").Key("app_name").String()
		instance.LogLevel = cfg.Section("").Key("log_level").String()
		instance.AdminPath = cfg.Section("").Key("admin_path").String()
		instance.Mysql.IP = cfg.Section("mysql").Key("ip").String()
		instance.Mysql.Port = cfg.Section("mysql").Key("port").String()
		instance.Mysql.User = cfg.Section("mysql").Key("user").String()
		instance.Mysql.Password = cfg.Section("mysql").Key("password").String()
		instance.Mysql.Database = cfg.Section("mysql").Key("database").String()
		instance.Redis.IP = cfg.Section("redis").Key("ip").String()
		instance.Redis.Port = cfg.Section("redis").Key("port").String()
		// 读取whitelist部分
		instance.Whitelist = cfg.Section("whitelist").Key("items").Strings(",")
		// 读取tenant部分
		instance.Tenant.auth = cfg.Section("tenant").Key("auth").String()
	})
	return getValueByKey(key)
}

func getValueByKey(key string) string {
	keys := strings.Split(key, ".")
	if len(keys) < 2 {
		return ""
	}
	section := keys[0]
	key = keys[1]
	switch section {
	case "app":
		return getAppValue(key)
	case "mysql":
		return getMysqlValue(key)
	case "redis":
		return getRedisValue(key)
	case "whitelist": // 新增case，用于处理whitelist部分
		return getWhitelistValue("items")
	case "tenant":
		return getTenantValue(key)
	default:
		return ""
	}
}

func getTenantValue(key string) string {
	switch key {
	case "auth":
		return instance.Tenant.auth
	default:
		return ""
	}
}

func getAppValue(key string) string {
	switch key {
	case "app_name":
		return instance.AppName
	case "log_level":
		return instance.LogLevel
	case "admin_path":
		return instance.AdminPath
	default:
		return ""
	}
}

func getMysqlValue(key string) string {
	switch key {
	case "ip":
		return instance.Mysql.IP
	case "port":
		return instance.Mysql.Port
	case "user":
		return instance.Mysql.User
	case "password":
		return instance.Mysql.Password
	case "database":
		return instance.Mysql.Database
	default:
		return ""
	}
}

func getRedisValue(key string) string {
	switch key {
	case "ip":
		return instance.Redis.IP
	case "port":
		return instance.Redis.Port
	default:
		return ""
	}
}

func getWhitelistValue(key string) string {
	// 假设whitelist部分只有一个键值对，即items
	if key == "items" {
		return strings.Join(instance.Whitelist, ",")
	}
	return ""
}
