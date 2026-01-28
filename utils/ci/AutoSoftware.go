package ci

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

var (
	softwareApp         string
	softwareMiddlewares map[string]interface{}
	softwareControllers map[string]interface{}
	softwareModules     map[string]interface{}
	softwareServices    map[string]interface{}
)

// 在包初始化时调用 SoftwareInit
func init() {
	SoftwareInit()
}

func SoftwareInit() {
	softwareApp = ""
	softwareMiddlewares = make(map[string]interface{})
	softwareControllers = make(map[string]interface{})
	softwareModules = make(map[string]interface{})
	softwareServices = make(map[string]interface{})
}

// GetControllerPrefixRegex 使用正则表达式从路径中提取"controllers"前的元素
// 支持多种路径格式，包括：
// - github.com/qinuoyun/shop/controllers/admin
// - /path/to/project/controllers/api
// - shop/controllers/...
func GetControllerPrefixRegex(path string) (string, error) {
	// 正则表达式说明：
	// ([^/]+)    捕获最后一个路径元素（非斜杠字符）
	// /controllers  匹配/controllers文本
	// ($|/)     确保controllers后是路径结尾或另一个斜杠
	re := regexp.MustCompile(`([^/]+)/controllers($|/)`)

	matches := re.FindStringSubmatch(path)
	if len(matches) < 2 {
		return "", fmt.Errorf("未找到符合模式的内容: %s", path)
	}

	return matches[1], nil
}

// SetSoftwareApp 设置软件名称
func SetSoftwareApp(appName string) {
	softwareApp = appName
}

// BinController 绑定控制器
func BinController(controller interface{}, PkgPathStr string) bool {
	v := reflect.ValueOf(controller)
	if v.Kind() == reflect.Ptr {
		v = v.Elem() // 获取指针指向的实际对象
	}

	// 获取控制器类型名称（包含包路径）
	ctrlTypeName := v.Type().String()

	module := extractModuleName(ctrlTypeName)

	fullPath := PkgPathStr + module

	//fmt.Printf("AAAA***============查看完整路径%v\n", fullPath)

	// 检查 PkgPathStr 是否已存在
	if _, exists := softwareControllers[fullPath]; exists {
		fmt.Printf("警告: 路径 %s 已存在，跳过绑定\n", fullPath)
		return false
	}

	vbf := reflect.ValueOf(controller)
	// 非模型或无方法则直接返回
	if vbf.NumMethod() == 0 {
		return false
	}

	projectName, err := GetControllerPrefixRegex(fullPath)
	if err != nil {
		fmt.Println("错误:", err)
		return false
	}

	softwareApp = projectName
	//fmt.Printf("获得项目名称[%v]获取相应路径: %v\n", projectName, fullPath)

	// 存入 Map 列表
	softwareControllers[fullPath] = controller
	return true
}

// SoftwareName 获取软件名称
func SoftwareName(table string) string {
	return fmt.Sprintf("ci_"+softwareApp+"_%s", table)
}

// BinModule 绑定模型
func BinModule(module interface{}) bool {
	vbf := reflect.ValueOf(module)
	// 非模型或无方法则直接返回
	if vbf.NumMethod() == 0 {
		return false
	}
	// 获取模型名称，并且去除 * 号的设置
	cleanedName := RemoveStarFromTypeName(module)
	// fmt.Printf("获得模型的路径%s", cleanedName)
	// 存入 Map 列表
	softwareModules[cleanedName] = module
	return true
}

// BinService 绑定服务
func BinService(service interface{}) bool {
	t := reflect.TypeOf(service)
	var cleanedName string
	// 2. 区分指针类型和非指针类型,分别获取名称
	switch t.Kind() {
	case reflect.Ptr:
		cleanedName = t.Elem().Name()
	default:
		return false
	}
	//fmt.Printf("3.获得模型的路径%s", cleanedName)
	// 存入 Map 列表
	softwareServices[cleanedName] = service
	return true
}

// BinMiddleware 绑定中间件（修正注释描述，原注释写的"绑定服务"）
func BinMiddleware(middleware interface{}) bool {
	t := reflect.TypeOf(middleware)
	var cleanedName string
	// 2. 区分指针类型和非指针类型，分别获取名称
	switch t.Kind() {
	case reflect.Ptr:
		cleanedName = t.Elem().Name()
	default:
		return false
	}
	// 存入 Map 列表
	softwareMiddlewares[cleanedName] = middleware
	return true
}

// GetMiddlewaresList 获取所有已注册的中间件（修正注释描述，原注释写的"获取所有已注册的服务"）
func GetMiddlewaresList() []interface{} {
	var middlewares []interface{}
	for _, middleware := range softwareMiddlewares {
		middlewares = append(middlewares, middleware)
	}
	return middlewares
}

// GetControllersList 获取所有已注册的控制器
func GetControllersList() map[string]interface{} {
	// 直接返回 softwareControllers
	return softwareControllers
}

// GetModulesList 获取所有已注册的模块
func GetModulesList() []interface{} {
	var modules []interface{}
	for _, module := range softwareModules {
		modules = append(modules, module)
	}
	return modules
}

// GetServicesList 获取所有已注册的服务
func GetServicesList() []interface{} {
	var services []interface{}
	for _, service := range softwareServices {
		services = append(services, service)
	}
	return services
}

func extractGroupName(fullName string) string {
	// 查找第一个点号的位置
	dotIndex := strings.Index(fullName, ".")
	if dotIndex == -1 {
		// 如果没有点号，返回整个字符串（可能是不包含包名的简单名称）
		return fullName
	}

	// 返回点号之前的部分
	return fullName[:dotIndex]
}

// 从控制器类型名称中提取模块名
func extractModuleName(typeName string) string {
	// 移除包路径，只保留类型名
	if dotIndex := strings.LastIndex(typeName, "."); dotIndex != -1 {
		typeName = typeName[dotIndex+1:]
	}

	// 移除"Controller"后缀
	typeName = strings.TrimSuffix(typeName, "Controller")

	// 特殊处理：Index控制器映射到根路径
	if typeName == "Index" {
		return "/"
	}

	// 转换为小写并添加斜杠
	return "/" + strings.ToLower(typeName) + "/"
}
