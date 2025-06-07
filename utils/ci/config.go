package ci

import (
	"fmt"
	"gopkg.in/ini.v1"
	"log"
	"os"
	"path/filepath"
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
	// 检查是否为开发环境（通过环境变量或命令行参数判断）
	isDev := os.Getenv("APP_ENV") == "development"
	var configPath string // 只需要配置文件路径，cacheDir用不到可以删除

	if isDev {
		// 开发环境：使用相对路径（基于当前工作目录）
		configPath = "config.ini" // 假设开发时config.ini在项目根目录
	} else {
		// 生产环境：使用可执行文件所在目录
		exePath, err := os.Executable()
		if err != nil {
			fmt.Printf("获取可执行文件路径失败: %v\n", err)
			// 发生错误时可以考虑使用默认路径或者退出
			configPath = "config.ini" // 默认回退路径
		} else {
			exeDir := filepath.Dir(exePath)
			configPath = filepath.Join(exeDir, "config.ini")
		}
	}
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
