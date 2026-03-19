package ci

import (
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
)

var (
	once       sync.Once
	instance   *Config
	configPath string
)

type Config struct {
	data map[string]map[string]interface{} // 动态存储所有配置
	cfg  *ini.File                         // 保存ini.File引用以便动态访问（仅 ini 时有效）
}

func C(key string) string {
	once.Do(loadConfig)
	return getValueByKey(key)
}

// loadConfig 优先读取 config.yaml，不存在则读取 config.ini
func loadConfig() {
	instance = &Config{
		data: make(map[string]map[string]interface{}),
	}

	yamlPath := GetConfigPath("config.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		if loadFromYaml(yamlPath) {
			configPath = yamlPath
			return
		}
	}

	// 回退到 config.ini
	iniPath := GetConfigPath("config.ini")
	cfg, err := ini.Load(iniPath)
	if err != nil {
		log.Fatalf("Fail to read config file: %v", err)
	}
	instance.cfg = cfg
	configPath = iniPath

	for _, section := range cfg.Sections() {
		sectionName := section.Name()
		if sectionName == "DEFAULT" {
			sectionName = "app"
		}
		if instance.data[sectionName] == nil {
			instance.data[sectionName] = make(map[string]interface{})
		}
		for _, key := range section.Keys() {
			keyName := key.Name()
			if strings.Contains(key.String(), ",") || keyName == "items" {
				instance.data[sectionName][keyName] = key.Strings(",")
			} else {
				instance.data[sectionName][keyName] = key.String()
			}
		}
	}
	if defaultSec, err := cfg.GetSection("DEFAULT"); err == nil && defaultSec != nil {
		if instance.data["app"] == nil {
			instance.data["app"] = make(map[string]interface{})
		}
		for _, key := range defaultSec.Keys() {
			keyName := key.Name()
			if _, exists := instance.data["app"][keyName]; !exists {
				if strings.Contains(key.String(), ",") || keyName == "items" {
					instance.data["app"][keyName] = key.Strings(",")
				} else {
					instance.data["app"][keyName] = key.String()
				}
			}
		}
	}
}

// loadFromYaml 从 config.yaml 加载并填充 instance.data，与 ini 相同的 section.key 结构
func loadFromYaml(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		log.Printf("config.yaml parse error: %v, fallback to config.ini", err)
		return false
	}
	for sectionName, sectionVal := range raw {
		if sectionVal == nil {
			continue
		}
		sectionMap := toStrMap(sectionVal)
		if sectionMap == nil {
			continue
		}
		if instance.data[sectionName] == nil {
			instance.data[sectionName] = make(map[string]interface{})
		}
		for k, v := range sectionMap {
			instance.data[sectionName][k] = yamlValueToConfig(v)
		}
	}
	return true
}

// yamlValueToConfig 将 yaml 解析出的值转为与 ini 一致：string 或 []string
func yamlValueToConfig(v interface{}) interface{} {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []interface{}:
		ss := make([]string, 0, len(val))
		for _, item := range val {
			ss = append(ss, valueToString(item))
		}
		return ss
	case []string:
		return val
	case int, int64, int32:
		return strconv.FormatInt(toInt64(val), 10)
	case uint, uint64, uint32:
		return strconv.FormatUint(toUint64(val), 10)
	case float64, float32:
		return strconv.FormatFloat(toFloat64(val), 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return valueToString(val)
	}
}

func valueToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int, int64, int32:
		return strconv.FormatInt(toInt64(val), 10)
	case uint, uint64, uint32:
		return strconv.FormatUint(toUint64(val), 10)
	case float64, float32:
		return strconv.FormatFloat(toFloat64(val), 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return ""
	}
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case int32:
		return int64(val)
	default:
		return 0
	}
}

func toUint64(v interface{}) uint64 {
	switch val := v.(type) {
	case uint:
		return uint64(val)
	case uint64:
		return val
	case uint32:
		return uint64(val)
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	default:
		return 0
	}
}

// toStrMap 将 yaml 的 section 转为 map[string]interface{}（兼容 map[interface{}]interface{}）
func toStrMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out
	}
	return nil
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
