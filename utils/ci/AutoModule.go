package ci

import (
	"gorm.io/gorm"
	"reflect"
	"strings"
)

var modules map[string]interface{}

func init() {
	modules = make(map[string]interface{})
}

var _DB *gorm.DB

// DB 结构体，封装数据库操作
type DB struct {
	*gorm.DB
	DBName string
}

func SetDB(db *gorm.DB) {
	_DB = db
}

func RegisterModule(module interface{}, path string) bool {
	vbf := reflect.ValueOf(module)
	//非模型或无方法则直接返回
	if vbf.NumMethod() == 0 {
		return false
	}
	//获取模型名称，并且去除*号的设置
	cleanedName := removeStarFromTypeName(module)
	//存入Map列表
	modules[cleanedName] = module
	return true
}

func removeStarFromTypeName(module interface{}) string {
	ctrlName := reflect.TypeOf(module).String()
	// 检查 ctrlName 是否以 * 开头，如果是则去掉 * 号
	if len(ctrlName) > 0 && ctrlName[0] == '*' {
		ctrlName = ctrlName[1:]
	}
	return ctrlName
}

// GetModules 用于获取所有已注册的 modules
func GetModules() map[string]interface{} {
	return modules
}

// M NewDB 函数用于创建一个新的 DB 实例
func M(name string) *DB {
	// 将输入的 name 转换成首字母大写的格式相连
	if strings.Count(name, ".") == 0 {
		// 如果用户只输入一个部分，重复该部分
		name = FirstUpper(strings.ToLower(name)) + "." + FirstUpper(strings.ToLower(name))
	} else {
		parts := strings.Split(name, ".")
		for i, part := range parts {
			// 仅将每个部分的首字母大写
			parts[i] = FirstUpper(strings.ToLower(part))
		}
		name = strings.Join(parts, ".")
	}

	// 获取 modules 中对应的模型切片
	modelSlice := modules[name]

	// 创建 DB 结构体实例
	db := &DB{
		DB:     _DB,
		DBName: name,
	}

	// 对 _DB 进行 Model 操作
	db.DB = db.DB.Model(modelSlice)

	return db
}
