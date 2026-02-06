package ci

import (
	"context"
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

// GetTenantID 从 Gin 上下文中获取当前请求的 tenant_id。
// 在进入异步 goroutine 前调用此方法保存 tenantID，再在 goroutine 内用 TenantContext(tenantID) 做 DB 操作。
func GetTenantID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	v, _ := c.Get("tenant_id")
	if id, ok := v.(string); ok {
		return id
	}
	return ""
}

// TenantContext 返回带有 tenant_id 的 context，用于异步/后台任务中的 DB 操作。
// 用法：在 handler 里先 tenantID := ci.GetTenantID(c)，再在 goroutine 里 db := ci.D().WithContext(ci.TenantContext(tenantID))。
func TenantContext(tenantID string) context.Context {
	return context.WithValue(context.Background(), "tenant_id", tenantID)
}

// DBWithTenant 返回带有指定 tenant_id 的 DB 实例，用于异步方法中替代 GetDB(c)。
// 用法：go func() { db := ci.DBWithTenant(ci.GetTenantID(c)); ... }()
func DBWithTenant(tenantID string) *gorm.DB {
	return _DB.WithContext(TenantContext(tenantID))
}

// Go 启动一个带 tenant 上下文的 goroutine，自动传递 tenant_id。
// 用法：ci.Go(c, func(db *gorm.DB) { db.Create(&record) })
func Go(c *gin.Context, fn func(db *gorm.DB)) {
	tenantID := GetTenantID(c)
	go func() {
		db := DBWithTenant(tenantID)
		fn(db)
	}()
}

// GoWithContext 启动一个带 tenant 上下文的 goroutine，同时传递 context 用于取消控制。
// 用法：ci.GoWithContext(c, func(ctx context.Context, db *gorm.DB) { ... })
func GoWithContext(c *gin.Context, fn func(ctx context.Context, db *gorm.DB)) {
	tenantID := GetTenantID(c)
	go func() {
		ctx := TenantContext(tenantID)
		db := _DB.WithContext(ctx)
		fn(ctx, db)
	}()
}

// GoWait 启动一个带 tenant 上下文的 goroutine，并等待执行完成。
// 用法：err := ci.GoWait(c, func(db *gorm.DB) error { return db.Create(&record).Error })
func GoWait(c *gin.Context, fn func(db *gorm.DB) error) error {
	tenantID := GetTenantID(c)
	errCh := make(chan error, 1)
	go func() {
		db := DBWithTenant(tenantID)
		errCh <- fn(db)
	}()
	return <-errCh
}

// Run 在当前 goroutine 中使用带 tenant 的 DB 执行操作（非异步，用于统一写法）。
// 用法：ci.Run(c, func(db *gorm.DB) { db.Find(&list) })
func Run(c *gin.Context, fn func(db *gorm.DB)) {
	db := GetDB(c)
	if db != nil {
		fn(db)
	}
}

// Async 异步任务构建器，支持链式调用
type Async struct {
	tenantID string
	ctx      context.Context
}

// NewAsync 创建异步任务构建器
// 用法：ci.NewAsync(c).Go(func(db *gorm.DB) { ... })
func NewAsync(c *gin.Context) *Async {
	return &Async{
		tenantID: GetTenantID(c),
	}
}

// WithContext 设置自定义 context（用于超时/取消控制）
func (a *Async) WithContext(ctx context.Context) *Async {
	a.ctx = context.WithValue(ctx, "tenant_id", a.tenantID)
	return a
}

// Go 启动异步任务
func (a *Async) Go(fn func(db *gorm.DB)) {
	go func() {
		var db *gorm.DB
		if a.ctx != nil {
			db = _DB.WithContext(a.ctx)
		} else {
			db = DBWithTenant(a.tenantID)
		}
		fn(db)
	}()
}

// Wait 启动异步任务并等待完成
func (a *Async) Wait(fn func(db *gorm.DB) error) error {
	errCh := make(chan error, 1)
	go func() {
		var db *gorm.DB
		if a.ctx != nil {
			db = _DB.WithContext(a.ctx)
		} else {
			db = DBWithTenant(a.tenantID)
		}
		errCh <- fn(db)
	}()
	return <-errCh
}

// DB 获取带 tenant 的 DB 实例（用于手动控制 goroutine）
func (a *Async) DB() *gorm.DB {
	if a.ctx != nil {
		return _DB.WithContext(a.ctx)
	}
	return DBWithTenant(a.tenantID)
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
// 支持两种调用方式：
//  1. 通过模型名称：M("models.expert")
//  2. 直接传入模型实例：M(&models.Expert{})
func M(model interface{}) *DB {
	return newDB(model, _DB)
}

// MT 创建带 tenant 的 DB 实例，用于异步任务。
// 用法：
//
//	tenantID := ci.GetTenantID(c)
//	go func() {
//	    ci.MT(tenantID, &models.Expert{}).Where(...).Find(&list)
//	}()
func MT(tenantID string, model interface{}) *DB {
	return newDB(model, DBWithTenant(tenantID))
}

// newDB 内部函数，创建 DB 实例
func newDB(model interface{}, baseDB *gorm.DB) *DB {
	var name string

	switch v := model.(type) {
	case string:
		name = v
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
	default:
		// 非字符串时，按类型生成模块名（与 RegisterModule 一致）
		name = RemoveStarFromTypeName(model)
	}

	// 获取 modules 中对应的模型切片
	modelSlice := modules[name]

	// 创建 DB 结构体实例
	db := &DB{
		DB:     baseDB,
		DBName: name,
	}

	// 对 baseDB 进行 Model 操作
	db.DB = db.DB.Model(modelSlice)

	return db
}
