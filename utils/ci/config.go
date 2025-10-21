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
	data map[string]map[string]interface{} // 动态存储所有配置
	cfg  *ini.File                         // 保存ini.File引用以便动态访问
}

func C(key string) string {
	configPath = GetConfigPath("config.ini")
	once.Do(func() {
		instance = &Config{
			data: make(map[string]map[string]interface{}),
		}
		// 使用计算得到的configPath而不是硬编码路径
		cfg, err := ini.Load(configPath)
		if err != nil {
			log.Fatalf("Fail to read file: %v", err)
		}
		instance.cfg = cfg

		// 动态读取所有section和key
		for _, section := range cfg.Sections() {
			sectionName := section.Name()
			if sectionName == "DEFAULT" {
				sectionName = "app" // 将默认section命名为app
			}
			if instance.data[sectionName] == nil {
				instance.data[sectionName] = make(map[string]interface{})
			}
			for _, key := range section.Keys() {
				keyName := key.Name()
				// 检查是否是逗号分隔的列表
				if strings.Contains(key.String(), ",") || keyName == "items" {
					instance.data[sectionName][keyName] = key.Strings(",")
				} else {
					instance.data[sectionName][keyName] = key.String()
				}
			}
		}
	})
	return getValueByKey(key)
}

func getValueByKey(key string) string {
	keys := strings.Split(key, ".")
	if len(keys) < 2 {
		return ""
	}
	section := keys[0]
	keyName := keys[1]

	// 动态从map中获取值
	if sectionData, ok := instance.data[section]; ok {
		if value, exists := sectionData[keyName]; exists {
			// 处理不同类型的值
			switch v := value.(type) {
			case string:
				return v
			case []string:
				return strings.Join(v, ",")
			default:
				return ""
			}
		}
	}
	return ""
}

// GetSection 获取整个section的所有配置
func GetSection(section string) map[string]interface{} {
	if instance == nil {
		C("app.app_name") // 触发初始化
	}
	if sectionData, ok := instance.data[section]; ok {
		return sectionData
	}
	return nil
}

// GetAllConfig 获取所有配置
func GetAllConfig() map[string]map[string]interface{} {
	if instance == nil {
		C("app.app_name") // 触发初始化
	}
	return instance.data
}
