package common

import (
	"fmt"
	"github.com/qinuoyun/caleyi/utils/ci"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"log"
	"os"
	"reflect"
	"strings"
	"time"
)

// SetTableNameForGORM 设置表名的函数
func SetTableNameForGORM(db *gorm.DB, obj interface{}, tableName string) *gorm.DB {
	return db.Table(tableName).Model(obj)
}

// ConvertToTableName 将类型字符串转换为表名
func ConvertToTableName(typeStr string) string {
	// 1. 去掉前面的*
	if strings.HasPrefix(typeStr, "*") {
		typeStr = typeStr[1:]
	}

	// 2. 使用中间的点分割字符串
	parts := strings.SplitN(typeStr, ".", 2)
	if len(parts) != 2 {
		return "" // 格式错误，返回空字符串或处理错误
	}
	A, B := parts[0], parts[1]

	// 3. 判断A和B是否相等
	var tableName string
	if A == B {
		tableName = A
	} else {
		tableName = A + "_" + B
	}

	// 4. 全部转换为小写并添加前缀
	return strings.ToLower(tableName)
}

func InitModule() {
	//读取配置文件
	ip := ci.C("mysql.ip")
	port := ci.C("mysql.port")
	user := ci.C("mysql.user")
	password := ci.C("mysql.password")
	database := ci.C("mysql.database")

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Error,
			Colorful:      true,
		},
	)

	options := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "pre_", // 表前缀
			SingularTable: true,   // 禁用表名复数
		},
		Logger: newLogger,
	}
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=utf8mb4&parseTime=True&loc=Local", user, password, ip, port, database)
	_DB, err := gorm.Open(mysql.Open(dsn), options)
	if err != nil {
		log.Fatal(err)
	}

	// 打开 DB 的 Debug 日志
	_DB.Debug()

	// 获取所有模块
	moduleMap := ci.GetModules()

	// 循环遍历并打印
	for _, value := range moduleMap {
		//fmt.Printf("Type of value: %T, Value: %v\n", value, value)
		if err := _DB.AutoMigrate(value); err != nil {
			log.Fatal(err)
		}
	}

	//获得所有插件模板
	for appName, modules := range ModulesPool {
		fmt.Printf("AppName: %s\n", appName)
		for _, module := range modules {
			// 通过反射获取类型名称
			moduleType := reflect.TypeOf(module)
			moduleName := moduleType.Name()

			if moduleName == "" {
				// 处理匿名结构体或指针类型
				moduleName = moduleType.String()
			}

			tableName := fmt.Sprintf("ci_%s_%s", appName, ConvertToTableName(moduleName))

			// 使用反射设置表名
			tx := SetTableNameForGORM(_DB, module, tableName)

			// 执行迁移或查询
			if err := tx.AutoMigrate(module); err != nil {
				log.Fatalf("迁移失败: %v", err)
			}
		}
		fmt.Println()
	}
	fmt.Println("===========================")

	// 将 _DB 设置到 ci 包中
	ci.SetDB(_DB)
}
