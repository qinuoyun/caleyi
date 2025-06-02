package ci

import (
	"gopkg.in/ini.v1"
	"log"
	"strings"
	"sync"
)

var (
	once     sync.Once
	instance *Config
)

type Config struct {
	AppName   string
	LogLevel  string
	AdminPath string
	Mysql     MysqlConfig
	Redis     RedisConfig
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
	once.Do(func() {
		instance = &Config{}
		cfg, err := ini.Load("./config.ini")
		if err != nil {
			log.Fatalf("Fail to read file: %v", err)
		}
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
