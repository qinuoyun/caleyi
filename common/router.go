package common

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/qinuoyun/caleyi/middleware"
	"github.com/qinuoyun/caleyi/utils/ci"
)

// HasMethodByReflect
// 通用反射检测函数，检测中间件是否实现了指定的方法（必须是导出方法，首字母大写）
// 支持指针接收者和值接收者两种方式
// methodName: 要检测的方法名，如 "HandleBefore" 或 "HandleAfter"
func HasMethodByReflect(obj interface{}, methodName string) (bool, reflect.Value) {
	// 1. 处理 nil 实例
	if obj == nil {
		fmt.Printf("  反射检测：obj 为 nil\n")
		return false, reflect.Value{}
	}

	// 2. 获取反射值对象和类型对象
	val := reflect.ValueOf(obj)
	typ := reflect.TypeOf(obj)
	if !val.IsValid() || typ == nil {
		fmt.Printf("  反射检测：反射对象无效\n")
		return false, reflect.Value{}
	}

	// 输出详细的类型信息用于调试
	fmt.Printf("  反射调试：查找方法=%s, val.Kind()=%v, typ=%v\n", methodName, val.Kind(), typ)
	fmt.Printf("  反射调试：val.NumMethod()=%d\n", val.NumMethod())
	for i := 0; i < val.NumMethod(); i++ {
		fmt.Printf("    方法[%d]: %s\n", i, val.Type().Method(i).Name)
	}

	var method reflect.Value
	// 3. 分场景强制查找方法（指针 → 值类型，层层兜底）
	// 场景1：先查找当前实例（指针/值）的方法（指针接收者方法）
	method = val.MethodByName(methodName)
	if method.IsValid() {
		fmt.Printf("  ✓ 反射检测：在指针类型上找到 %s 方法（指针接收者）\n", methodName)
		goto checkSignature // 找到方法，直接校验签名
	}
	fmt.Printf("  反射调试：场景1未找到方法\n")

	// 场景2：当前实例未找到，若为指针则取值类型再查找（值接收者方法）
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			fmt.Printf("  反射检测：指针实例为 nil，无法取值\n")
			return false, reflect.Value{}
		}
		elemVal := val.Elem()
		fmt.Printf("  反射调试：场景2解引用后 elemVal.Kind()=%v, elemVal.NumMethod()=%d\n", elemVal.Kind(), elemVal.NumMethod())
		for i := 0; i < elemVal.NumMethod(); i++ {
			fmt.Printf("    值类型方法[%d]: %s\n", i, elemVal.Type().Method(i).Name)
		}
		method = elemVal.MethodByName(methodName)
		if method.IsValid() {
			fmt.Printf("  ✓ 反射检测：在值类型上找到 %s 方法（值接收者）\n", methodName)
			goto checkSignature // 找到方法，直接校验签名
		}
		fmt.Printf("  反射调试：场景2未找到方法\n")
	}

	// 场景3：若为值类型，尝试通过类型方法集查找（兼容边界情况）
	if val.Kind() != reflect.Ptr {
		ptrVal := reflect.New(typ)
		fmt.Printf("  反射调试：场景3创建指针包装 ptrVal.NumMethod()=%d\n", ptrVal.NumMethod())
		method = ptrVal.MethodByName(methodName)
		if method.IsValid() {
			fmt.Printf("  ✓ 反射检测：通过指针包装找到 %s 方法\n", methodName)
			// 注意：此时需要重新绑定实例到原始对象
			ptrVal.Elem().Set(val)
			method = ptrVal.MethodByName(methodName)
			goto checkSignature
		}
		fmt.Printf("  反射调试：场景3未找到方法\n")
	}

	// 场景4：所有场景都未找到方法
	fmt.Printf("  ✗ 反射检测：未找到 %s 方法\n", methodName)
	return false, reflect.Value{}

	// 4. 严格校验方法签名
checkSignature:
	methodType := method.Type()
	// 4.1 校验参数数量：仅1个业务参数（*gin.Context）
	if methodType.NumIn() != 1 {
		fmt.Printf("  反射检测：%s 参数数量不符，预期1个，实际%d个\n", methodName, methodType.NumIn())
		return false, reflect.Value{}
	}
	// 4.2 校验参数类型：必须是 *gin.Context
	contextType := reflect.TypeOf(&gin.Context{})
	if methodType.In(0) != contextType {
		fmt.Printf("  反射检测：%s 参数类型不符，预期*gin.Context，实际%s\n", methodName, methodType.In(0))
		return false, reflect.Value{}
	}
	// 4.3 校验返回值数量：必须为0
	if methodType.NumOut() != 0 {
		fmt.Printf("  反射检测：%s 返回值数量不符，预期0个，实际%d个\n", methodName, methodType.NumOut())
		return false, reflect.Value{}
	}

	return true, method
}

// RegisterMiddlewareHandlers
// 统一注册中间件的 HandleBefore 和 HandleAfter 方法
// stage: "before" 表示注册前置中间件，"after" 表示注册后置中间件
func RegisterMiddlewareHandlers(R *gin.Engine, middlewareList []interface{}, stage string) {
	var methodName string
	if stage == "before" {
		methodName = "HandleBefore"
		fmt.Printf("\n========== 开始注册 HandleBefore 中间件 ==========\n")
	} else if stage == "after" {
		methodName = "HandleAfter"
		fmt.Printf("\n========== 开始注册 HandleAfter 中间件 ==========\n")
	} else {
		fmt.Printf("未知的注册阶段：%s\n", stage)
		return
	}

	for i, mw := range middlewareList {
		fmt.Printf("\n=== 处理中间件（索引：%d，类型：%T，阶段：%s）===\n", i, mw, stage)

		// 反射检测是否存在有效方法
		hasMethod, methodVal := HasMethodByReflect(mw, methodName)
		if !hasMethod {
			fmt.Printf("  该中间件不存在有效 %s 方法，跳过注册\n", methodName)
			continue
		}

		// 捕获当前循环的 methodVal，解决闭包作用域覆盖问题
		validMethod := methodVal
		fmt.Printf("  该中间件存在 %s 方法，开始注册\n", methodName)

		// 封装为 Gin 中间件并注册
		R.Use(func(c *gin.Context) {
			// 调用前终极校验
			if !validMethod.IsValid() {
				fmt.Printf("  警告：%s 方法无效，跳过执行\n", methodName)
				return
			}
			if c == nil {
				fmt.Printf("  警告：gin.Context 为 nil，跳过执行\n")
				return
			}

			// 准备参数并安全调用
			params := []reflect.Value{reflect.ValueOf(c)}
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("  调用 %s 异常：%v\n", methodName, r)
				}
			}()
			validMethod.Call(params)
		})
	}
	fmt.Printf("\n========== %s 中间件注册完成 ==========\n\n", methodName)
}

func InitRouter() *gin.Engine {
	//初始化路由
	R := gin.Default()
	err := R.SetTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		return nil
	}
	//访问公共目录
	R.Static("/static", "./static")
	R.Static("/public", "./public")
	R.Static("/uploads", "./uploads")
	R.Static("/web", "./views/web")

	// 处理静态文件和默认页面
	R.GET("/admin/*any", func(c *gin.Context) {
		filePath := c.Param("any")
		if filePath == "" || strings.HasSuffix(filePath, "/") {
			filePath = "index.html"
		}
		fullPath := fmt.Sprintf("views/admin/%s", filePath)
		if _, err := os.Stat(fullPath); err == nil {
			c.File(fullPath)
		} else {
			c.File("views/admin/index.html")
		}
	})

	// 处理静态文件和默认页面
	R.GET("/merchant/*any", func(c *gin.Context) {
		filePath := c.Param("any")
		if filePath == "" || strings.HasSuffix(filePath, "/") {
			filePath = "index.html"
		}
		fullPath := fmt.Sprintf("views/merchant/%s", filePath)
		if _, err := os.Stat(fullPath); err == nil {
			c.File(fullPath)
		} else {
			c.File("views/merchant/index.html")
		}
	})

	//访问域名根目录重定向
	R.GET("/", func(c *gin.Context) {
		// 检查 views/web/index.html 是否存在
		if _, err := os.Stat("views/web/index.html"); err == nil {
			// 文件存在，渲染 HTML
			c.File("views/web/index.html")
		} else {
			// 文件不存在，返回 JSON
			c.JSON(200, gin.H{"code": 200, "message": "欢迎使用卡莱易框架"})
		}
	})

	R.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"X-Requested-With", "Content-Type", "Authorization", "BusinessId", "verify-encrypt", "ignoreCancelToken", "verify-time"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 获取原始中间件切片（不做任何转换）
	middlewareList := ci.GetMiddlewaresList()
	fmt.Printf("原始 middlewareList 长度：%d\n", len(middlewareList))
	for i, item := range middlewareList {
		fmt.Printf("  原始索引 %d：类型=%T，值=%+v，是否nil=%v\n", i, item, item, item == nil)
	}

	// 注册 HandleBefore 中间件（前置处理）
	RegisterMiddlewareHandlers(R, middlewareList, "before")

	//4.验证token
	R.Use(middleware.JwtVerify)

	//5.处理租户问题
	R.Use(middleware.TenantVerify)

	// 注册 HandleAfter 中间件（后置处理）
	RegisterMiddlewareHandlers(R, middlewareList, "after")

	//7.找不到路由
	R.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method
		c.JSON(404, gin.H{"code": 404, "message": "您" + method + "请求地址：" + path + "不存在！"})
	})

	//绑定基本路由，访问路径：/User/List
	ci.Bind(R)
	//绑定插件路由
	BindSoftwareRoutes(R)
	//返回实例
	return R
}
