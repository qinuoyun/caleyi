package ci

import (
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

// SetDB 设置DB
func SetDB(db *gorm.DB) {
	_DB = db
}

// D 获取全局数据库连接
func D() *gorm.DB {
	return _DB
}

// GetDB 从 Gin 上下文中获取 GORM 数据库实例
// 如果获取失败，会直接通过 c.JSON 返回错误信息并终止请求处理
// 如果获取成功，返回 GORM 数据库实例
func GetDB(c *gin.Context) *gorm.DB {
	dbValue, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"error": "Database instance not found"})
		c.Abort()
		return nil
	}

	gormDB, ok := dbValue.(*gorm.DB)
	if !ok {
		c.JSON(500, gin.H{"error": "Database instance type error"})
		c.Abort()
		return nil
	}

	return gormDB
}

// RegisterModule 注册系统模型
func RegisterModule(module interface{}, path string) bool {
	vbf := reflect.ValueOf(module)
	//非模型或无方法则直接返回
	if vbf.NumMethod() == 0 {
		return false
	}
	//获取模型名称，并且去除*号的设置
	cleanedName := RemoveStarFromTypeName(module)
	//存入Map列表
	modules[cleanedName] = module
	return true
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
