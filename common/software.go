package common

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

// Controller 定义控制器模块接口
type Controller interface{}

// Module 定义业务模块接口
type Module interface{}

// Service 定义服务层接口
type Service interface{}

// Software 定义完整插件接口
type Software interface {
	// Init 初始化插件（传入配置文件路径）
	Init(configPath string) error
	GetControllers() []Controller
	GetModules() []Module
	GetServices() []Service
}

// Route 路由结构体
type Route struct {
	path       string         //url路径
	httpMethod string         //http方法 get post
	Method     reflect.Value  //方法路由
	Args       []reflect.Type //参数类型
}

// Routes 路由集合
var Routes []Route

// ModulesPool 全局模块池 - 按AppName分组存储模块
var ModulesPool = make(map[string][]Module)

// ConvertToControllers 将通用接口类型转换为 Controller 类型
func ConvertToControllers(controllers map[string]interface{}, AppName string) []Controller {
	var result []Controller
	for route, c := range controllers {
		if Controller, ok := c.(Controller); ok {
			//fmt.Printf("  成功转换控制器: %T\n", c) // 新增调试日志
			result = append(result, Controller)
			registerControllerRoutes(c, route, AppName)
		} else {
			fmt.Printf("  转换失败: %T\n", c) // 新增调试日志
		}
	}
	return result
}

// ConvertToModules 将通用接口类型转换为 Module 类型
func ConvertToModules(modules []interface{}, AppName string) []Module {
	var result []Module
	for _, m := range modules {
		if module, ok := m.(Module); ok {
			result = append(result, module)
			//fmt.Printf("  成功转换模块: %T\n", module) // 新增调试日志
		} else {
			fmt.Printf("  转换失败: %T\n", m) // 新增调试日志
		}
	}
	ModulesPool[AppName] = result
	return result
}

// ConvertToServices 将通用接口类型转换为 Service 类型
func ConvertToServices(services []interface{}) []Service {
	var result []Service
	for _, s := range services {
		if svc, ok := s.(Service); ok {
			result = append(result, svc)
		}
	}
	return result
}

// BindSoftwareRoutes Bind 绑定路由 m是方法GET POST等
func BindSoftwareRoutes(e *gin.Engine) {
	//fmt.Printf("====我是插件路由-查看路径名称%v\n", Routes)
	for _, route := range Routes {
		//fmt.Printf("查看路径名称%v\n", route.path)
		//只允许GET或者POST
		e.Match([]string{"GET", "POST"}, route.path, matchPath(route.path, route))
	}
}

// 根据path匹配对应的方法
func matchPath(path string, route Route) gin.HandlerFunc {
	return func(c *gin.Context) {
		fields := strings.Split(path, "/")
		//fmt.Println("00000-fields,len(fields)=", fields, len(fields))
		if len(fields) < 4 {
			return
		}
		if len(Routes) > 0 {
			arguments := make([]reflect.Value, 1)
			arguments[0] = reflect.ValueOf(c) // *gin.Context
			route.Method.Call(arguments)
		}
	}
}

// GetAdminMerchantPathByRegex 通过正则表达式匹配目标子串
func GetAdminMerchantPathByRegex(input string) string {
	// 编译正则表达式：匹配 controllers/ 后的所有字符
	reg := regexp.MustCompile(`controllers/(.+)`)
	// 查找匹配项
	matchResult := reg.FindStringSubmatch(input)
	if len(matchResult) >= 2 {
		return matchResult[1]
	}
	return "" // 无匹配时返回空
}

// 从控制器对象自动注册路由
func registerControllerRoutes(controller interface{}, route string, prefix string) {
	v := reflect.ValueOf(controller)
	if v.Kind() == reflect.Ptr {
		v = v.Elem() // 获取指针指向的实际对象
	}

	fullPath := GetAdminMerchantPathByRegex(route)

	// 遍历控制器的所有方法
	for i := 0; i < v.NumMethod(); i++ {
		method := v.Method(i)
		methodName := v.Type().Method(i).Name

		// 跳过非公共方法（小写开头）
		if !isPublicMethod(methodName) {
			continue
		}
		// 生成路由路径：根路径 + 模块名 + 方法名（首字母小写）
		path := "/api/" + prefix + "/" + fullPath + firstLower(methodName)
		//fmt.Printf("============查看路径名称%v\n", path)

		// 根据方法名前缀自动推断HTTP方法
		httpMethod := inferHTTPMethod(methodName)

		// 收集方法参数类型（用于依赖注入）
		paramTypes := collectMethodParams(method)

		// 注册主路由
		registerRoute(path, method, paramTypes, httpMethod)
	}
}

// 判断方法是否为公共方法（首字母大写）
func isPublicMethod(methodName string) bool {
	return len(methodName) > 0 && unicode.IsUpper(rune(methodName[0]))
}

// 推断HTTP方法
func inferHTTPMethod(methodName string) string {
	switch {
	case methodName == "Index" ||
		strings.HasPrefix(methodName, "Get") && !strings.HasPrefix(methodName, "GetPost"):
		return "GET"
	case strings.HasPrefix(methodName, "Del") || methodName == "Del":
		return "DELETE"
	case strings.HasPrefix(methodName, "Put") || methodName == "Put":
		return "PUT"
	default:
		return "POST" // 默认使用POST
	}
}

// 收集方法的参数类型
func collectMethodParams(method reflect.Value) []reflect.Type {
	paramTypes := make([]reflect.Type, 0, method.Type().NumIn())
	for j := 0; j < method.Type().NumIn(); j++ {
		paramTypes = append(paramTypes, method.Type().In(j))
	}
	return paramTypes
}

// 注册单条路由
func registerRoute(path string, method reflect.Value, params []reflect.Type, httpMethod string) {
	route := Route{
		path:       path,
		Method:     method,
		Args:       params,
		httpMethod: httpMethod,
	}
	Routes = append(Routes, route)
}

// 将字符串首字母转为小写
// 将驼峰命名的字符串转换为小写并用斜杠分隔
func firstLower(s string) string {
	if len(s) == 0 {
		return s
	}

	// 处理首字母小写
	result := strings.ToLower(s[:1]) + s[1:]

	// 使用正则表达式在大写字母前插入斜杠（但不处理第一个字符）
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	result = re.ReplaceAllString(result, "$1/$2")

	// 将所有字符转换为小写
	return strings.ToLower(result)
}
