package common

import (
	"fmt"
	"os"
	"path/filepath"
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
		//fmt.Printf("  反射检测：obj 为 nil\n")
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
	//	fmt.Printf("  反射调试：查找方法=%s, val.Kind()=%v, typ=%v\n", methodName, val.Kind(), typ)
	//	fmt.Printf("  反射调试：val.NumMethod()=%d\n", val.NumMethod())
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
	//	fmt.Printf("  反射调试：场景1未找到方法\n")

	// 场景2：当前实例未找到，若为指针则取值类型再查找（值接收者方法）
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			fmt.Printf("  反射检测：指针实例为 nil，无法取值\n")
			return false, reflect.Value{}
		}
		elemVal := val.Elem()
		//	fmt.Printf("  反射调试：场景2解引用后 elemVal.Kind()=%v, elemVal.NumMethod()=%d\n", elemVal.Kind(), elemVal.NumMethod())
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
		//		fmt.Printf("  反射调试：场景3创建指针包装 ptrVal.NumMethod()=%d\n", ptrVal.NumMethod())
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
func RegisterMiddlewareHandlers(R gin.IRoutes, middlewareList []interface{}, stage string) {
	var methodName string
	// 修复：恢复 methodName 赋值逻辑（原代码被注释导致方法名为空）
	switch stage {
	case "before":
		methodName = "HandleBefore"
		fmt.Printf("\n========== 开始注册 HandleBefore 中间件 ==========\n")
	case "after":
		methodName = "HandleAfter"
		fmt.Printf("\n========== 开始注册 HandleAfter 中间件 ==========\n")
	default:
		fmt.Printf("未知的注册阶段：%s\n", stage)
		return
	}

	// 注意：原代码循环变量错误（for mw := range middlewareList 应为 for i, mw := range）
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

// 定义多前端项目的路径映射（统一管理，方便维护）
var frontEndProjects = map[string]string{
	"/admin":    "./views/admin",    // /admin -> 后台管理项目
	"/h5":       "./views/h5",       // /h5 -> H5移动端项目
	"/merchant": "./views/merchant", // /merchant -> 商户端项目
	"/store":    "./views/store",    // /store -> 门店端项目
}

// 静态资源后缀列表（用于过滤，避免静态资源被history路由拦截）
var staticExts = []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".woff", ".woff2", ".ttf", ".map", ".json", ".txt"}

func InitRouter() *gin.Engine {
	//初始化路由
	R := gin.Default()
	err := R.SetTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		return nil
	}

	// ========== 原有静态资源配置（保留基础路径，子项目由下文history逻辑处理） ==========
	R.Static("/static", "./static")
	R.Static("/public", "./public")
	R.Static("/uploads", "./runtime/uploads")

	// ========== 重构：统一处理多项目history路由（替代原有零散的/admin、/merchant处理逻辑） ==========
	// 处理所有前端项目的GET请求，解决history刷新问题
	for prefix, distDir := range frontEndProjects {
		// 为每个项目前缀注册路由
		R.GET(prefix+"/*any", func(c *gin.Context) {
			// 获取通配符参数
			filePath := c.Param("any")
			// 确定目标项目目录
			targetDir := distDir

			// 1. 处理根路径特殊情况（/ 对应的any参数为空时）
			if prefix == "/" && (filePath == "" || filePath == "/") {
				fullPath := fmt.Sprintf("%s/index.html", targetDir)
				if _, err := os.Stat(fullPath); err == nil {
					c.File(fullPath)
				} else {
					// 保留原有根路径返回JSON的逻辑
					c.JSON(200, gin.H{"code": 200, "message": "欢迎使用卡莱易框架"})
				}
				return
			}

			// 2. 处理非根路径的情况（admin/h5/merchant/store）
			if filePath == "" || strings.HasSuffix(filePath, "/") {
				filePath = "index.html"
			}
			// 拼接完整文件路径（去除any参数开头的/，避免路径重复）
			fullPath := fmt.Sprintf("%s/%s", targetDir, strings.TrimPrefix(filePath, "/"))

			// 3. 判断是否为静态资源
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fullPath), "."))
			isStatic := false
			for _, e := range staticExts {
				if ext == strings.TrimPrefix(e, ".") {
					isStatic = true
					break
				}
			}

			// 4. 静态资源存在则返回，不存在则404；非静态资源则返回index.html（history路由）
			if isStatic {
				if _, err := os.Stat(fullPath); err == nil {
					c.File(fullPath)
				} else {
					c.Status(404)
				}
			} else {
				// 非静态资源请求，返回项目的index.html（解决history刷新）
				c.File(fmt.Sprintf("%s/index.html", targetDir))
			}
		})
	}

	// ========== 原有CORS中间件（保留） ==========
	R.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"X-Requested-With", "Content-Type", "Authorization", "BusinessId", "verify-encrypt", "ignoreCancelToken", "verify-time"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// ========== 原有中间件注册逻辑（保留） ==========
	// 获取原始中间件切片（不做任何转换）
	middlewareList := ci.GetMiddlewaresList()

	// 2. 创建 /api 路由组
	apiGroup := R.Group("/api")
	{
		// 第一步：注册 HandleBefore 前置中间件（仅 /api 生效）
		RegisterMiddlewareHandlers(apiGroup, middlewareList, "before")

		// 第二步：JWT 验证 + 租户验证（仅 /api 生效）
		apiGroup.Use(middleware.JwtVerify)
		apiGroup.Use(middleware.TenantVerify)

		// 第三步：注册 HandleAfter 后置中间件（仅 /api 生效）
		RegisterMiddlewareHandlers(apiGroup, middlewareList, "after")
	}

	// ========== 调整NoRoute逻辑：处理根项目的history路由和API 404 ==========
	R.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// 1. 判断是否为子前端项目路径（admin/h5/merchant/store）
		// 这些路径已经在 GET /*any 中注册，若走到此处说明是该项目内部的404资源
		for prefix := range frontEndProjects {
			if strings.HasPrefix(path, prefix) {
				c.Status(404)
				return
			}
		}

		// 2. 如果是 API 请求，返回 JSON 404
		if strings.HasPrefix(path, "/api") {
			c.JSON(404, gin.H{"code": 404, "message": "您" + method + "请求地址：" + path + "不存在！"})
			return
		}

		// 3. 处理根项目 (views/web) 的静态资源和 history 路由
		targetDir := "./views/web"
		// 拼接完整文件路径
		fullPath := filepath.Join(targetDir, strings.TrimPrefix(path, "/"))

		// 检查文件是否存在且不是目录
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			c.File(fullPath)
			return
		}

		// 非静态资源（无后缀或后缀不匹配），返回根项目的 index.html 以支持 history 模式
		// 如果是明确的静态资源请求（如 .js, .css）但文件不存在，则返回 404
		ext := strings.ToLower(filepath.Ext(path))
		isStatic := false
		for _, e := range staticExts {
			if ext == e {
				isStatic = true
				break
			}
		}

		if isStatic {
			c.Status(404)
		} else {
			// 返回根项目的 index.html
			indexPath := filepath.Join(targetDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				c.File(indexPath)
			} else if path == "/" || path == "" {
				// 如果连 index.html 都没有，且访问的是根路径，则返回欢迎信息
				c.JSON(200, gin.H{"code": 200, "message": "欢迎使用卡莱易框架"})
			} else {
				// 其他不存在的路径返回 404
				c.JSON(404, gin.H{"code": 404, "message": "您" + method + "请求地址：" + path + "不存在！"})
			}
		}
	})

	// ========== 原有路由绑定逻辑（保留） ==========
	//绑定基本路由，访问路径：/User/List
	ci.Bind(R)
	//绑定插件路由
	BindSoftwareRoutes(R, apiGroup)

	//返回实例
	return R
}
