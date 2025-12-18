package common

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/qinuoyun/caleyi/utils/ci"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres" // 导入 PostgreSQL 驱动
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func InitModule() {
	sqlType := ci.C("app.app_sql")
	var (
		// 声明变量，作用域覆盖整个函数
		ip, port, user, password, database string
		dialector                          gorm.Dialector // 统一驱动接口
	)

	// 根据 sqlType 读取对应配置并选择驱动
	switch sqlType {
	case "mysql":
		// 读取 MySQL 配置
		ip = ci.C("mysql.ip")
		port = ci.C("mysql.port")
		user = ci.C("mysql.user")
		password = ci.C("mysql.password")
		database = ci.C("mysql.database")
		// MySQL DSN 格式
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, ip, port, database)
		dialector = mysql.Open(dsn) // MySQL 驱动

	case "postgre", "postgres": // 兼容两种写法
		// 读取 PostgreSQL 配置
		ip = ci.C("pgsql.ip")
		port = ci.C("pgsql.port")
		user = ci.C("pgsql.user")
		password = ci.C("pgsql.password")
		database = ci.C("pgsql.database")
		// PostgreSQL DSN 格式（注意字段名和 MySQL 不同）
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
			ip, port, user, password, database)
		dialector = postgres.Open(dsn) // PostgreSQL 驱动

	default:
		log.Fatalf("不支持的数据库类型：%s（仅支持 mysql/postgre）", sqlType)
	}

	// 初始化 GORM 日志配置
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Error,
			Colorful:      true,
		},
	)

	// GORM 全局配置
	options := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "pre_", // 表前缀
			SingularTable: true,   // 禁用表名复数
		},
		Logger: newLogger,
	}

	// 连接数据库（使用统一的 dialector 接口）
	_DB, err := gorm.Open(dialector, options)
	if err != nil {
		log.Fatalf("数据库连接失败：%v", err)
	}

	// 打开 Debug 日志
	_DB.Debug()

	// 迁移模块（原逻辑保留）
	moduleMap := ci.GetModules()
	for _, value := range moduleMap {
		if err := _DB.AutoMigrate(value); err != nil {
			log.Fatalf("模块迁移失败：%v", err)
		}
	}

	// 迁移插件模板（原逻辑保留）
	for _, modules := range ModulesPool {
		for _, module := range modules {
			if err := _DB.AutoMigrate(module); err != nil {
				log.Fatalf("插件模板迁移失败：%v", err)
			}
		}
	}

	fmt.Println("===========================")
	fmt.Printf("数据库连接成功！类型：%s，数据库名：%s\n", sqlType, database)
	fmt.Println("===========================")

	// 将 DB 实例设置到 ci 包中
	ci.SetDB(_DB)
}
