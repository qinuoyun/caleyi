package ci

import (
	"context"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var modules map[string]interface{}

func init() {
	modules = make(map[string]interface{})
}

var _DB *gorm.DB

// goroutineDBMap 按 goroutine ID 存储当前请求的带 tenant 的 DB
var goroutineDBMap sync.Map

// getGoroutineID 获取当前 goroutine ID
func getGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// 格式："goroutine 123 [running]"
	s := string(buf[:n])
	s = s[len("goroutine "):]
	s = s[:strings.IndexByte(s, ' ')]
	id, _ := strconv.ParseUint(s, 10, 64)
	return id
}

// BindDB 将带 tenant 的 DB 绑定到当前 goroutine，供 M() 自动获取。
// 在中间件中调用，配合 defer UnbindDB() 使用。
func BindDB(db *gorm.DB) {
	goroutineDBMap.Store(getGoroutineID(), db)
}

// UnbindDB 清除当前 goroutine 绑定的 DB，防止内存泄漏。
func UnbindDB() {
	goroutineDBMap.Delete(getGoroutineID())
}

// currentDB 获取当前 goroutine 绑定的 DB，没有则返回全局 _DB
func currentDB() *gorm.DB {
	if v, ok := goroutineDBMap.Load(getGoroutineID()); ok {
		return v.(*gorm.DB)
	}
	return _DB
}

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
// goroutine 内 ci.M(m) 也能自动获取 tenant。
// 用法：ci.Go(c, func(db *gorm.DB) { db.Create(&record) })
func Go(c *gin.Context, fn func(db *gorm.DB)) {
	tenantID := GetTenantID(c)
	go func() {
		db := DBWithTenant(tenantID)
		BindDB(db)
		defer UnbindDB()
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
		BindDB(db)
		defer UnbindDB()
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
		BindDB(db)
		defer UnbindDB()
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

// GetModules 用于获取所有已注册的 modules（包含插件模块）
func GetModules() map[string]interface{} {
	// 合并插件注册的模块
	for _, module := range GetModulesList() {
		cleanedName := RemoveStarFromTypeName(module)
		if _, ok := modules[cleanedName]; !ok {
			modules[cleanedName] = module
		}
	}
	return modules
}

// GetModule 根据名称获取已注册的模型
// 用法：ci.GetModule("models.Expert") 或 ci.GetModule("expert")
func GetModule(name string) interface{} {
	return findModule(name)
}

// findModule 统一的模块查找逻辑，支持精确匹配和模糊匹配
func findModule(name string) interface{} {
	// 1. 精确匹配（如 "models.Expert"）
	if m, ok := modules[name]; ok {
		return m
	}

	// 2. 忽略大小写匹配
	nameLower := strings.ToLower(name)
	for key, m := range modules {
		// 完整 key 忽略大小写（如 "models.expert" 匹配 "models.Expert"）
		if strings.ToLower(key) == nameLower {
			return m
		}
		// 只匹配结构体名称（如 "expert" 匹配 "models.Expert"）
		parts := strings.Split(key, ".")
		structName := parts[len(parts)-1]
		if strings.ToLower(structName) == nameLower {
			return m
		}
	}

	return nil
}

// M 创建 DB 实例，自动获取当前请求的租户上下文。
// 在 HTTP 请求中无需传任何额外参数，中间件已自动绑定。
// 用法：ci.M(m).Where("id = ?", 1).First(&result)
func M(model interface{}) *DB {
	return newDB(model, currentDB())
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
	default:
		// 非字符串时，按类型生成模块名（与 RegisterModule 一致）
		name = RemoveStarFromTypeName(model)
	}

	// 查找 modules：先精确匹配，再模糊匹配
	modelSlice := findModule(name)

	// 创建 DB 结构体实例
	db := &DB{
		DB:     baseDB,
		DBName: name,
	}

	// 对 baseDB 进行 Model 操作
	db.DB = db.DB.Model(modelSlice)

	return db
}
