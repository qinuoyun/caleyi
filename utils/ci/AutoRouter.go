package ci

/**
* 自动路由工具
 */
import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
)

// Route 路由结构体
type Route struct {
	path       string         //url路径
	httpMethod string         //http方法 get post
	Method     reflect.Value  //方法路由
	Args       []reflect.Type //参数类型
}

// Routes 路由集合
var Routes []Route

// Register 注册控制器
func Register(controller interface{}, PkgPathStr string) bool {
	//fmt.Printf("日志[PkgPathStr]：%v\n", PkgPathStr)
	vbf := reflect.ValueOf(controller)
	//非控制器或无方法则直接返回
	if vbf.NumMethod() == 0 {
		return false
	}
	basePkg := "/api"
	rootPkg := ""

	if strings.Contains(PkgPathStr, "/app") {
		PkgPathArr := strings.Split(PkgPathStr, "/app")
		rootPkg = PkgPathArr[len(PkgPathArr)-1]
	}

	//fmt.Printf("A-日志[rootPkg]：%v\n", rootPkg)

	//重置URL地址链接
	rootPkg = basePkg + rootPkg

	//fmt.Println("查看根[rootPkg]=", rootPkg)
	ctrlName := reflect.TypeOf(controller).String()
	// 检查是否以 "Controller" 结尾
	if strings.HasSuffix(ctrlName, "Controller") {
		// 截取字符串
		ctrlName = strings.TrimSuffix(ctrlName, "Controller")
		// 如果类型名包含包名，还需要去除包名部分
		if dotIndex := strings.LastIndex(ctrlName, "."); dotIndex != -1 {
			ctrlName = ctrlName[dotIndex+1:]
		}
	}

	module := ctrlName
	if strings.Contains(ctrlName, ".") {
		module = ctrlName[strings.Index(ctrlName, ".")+1:]
	}
	//fmt.Println("module=", module)
	if module == "Index" { //去index
		module = "/"
	} else {
		module = "/" + strings.ToLower(module) + "/"
	}
	v := reflect.ValueOf(controller)
	// fmt.Println("遍历方法:")
	// fmt.Println(ctrlName)
	//遍历方法
	for i := 0; i < v.NumMethod(); i++ {
		method := v.Method(i)
		action := v.Type().Method(i).Name
		//拼接路由地址
		path := rootPkg + module + FirstLower(action)
		//遍历参数
		params := make([]reflect.Type, 0, v.NumMethod())
		httpMethod := "POST" //默认POST
		if (strings.HasPrefix(action, "Get") && !strings.HasPrefix(action, "GetPost")) || action == "Index" {
			httpMethod = "GET"
		} else if strings.HasPrefix(action, "Del") || action == "Del" {
			httpMethod = "DELETE"
		} else if strings.HasPrefix(action, "Put") || action == "Put" {
			httpMethod = "PUT"
		}
		for j := 0; j < method.Type().NumIn(); j++ {
			params = append(params, method.Type().In(j))
		}
		// fmt.Println("params=", params)
		// fmt.Println("action=", action)
		route := Route{path: path, Method: method, Args: params, httpMethod: httpMethod}
		Routes = append(Routes, route)
		if strings.HasPrefix(action, "GetPost") { //再增加一个get请求
			route := Route{path: path, Method: method, Args: params, httpMethod: "GET"}
			Routes = append(Routes, route)
		}
	}
	// fmt.Println("Routes=", Routes)
	return true
}

// Bind 绑定路由 m是方法GET POST等
func Bind(e *gin.Engine) {
	for _, route := range Routes {
		//只允许GET或者POST
		e.Match([]string{"GET", "POST"}, route.path, match(route.path, route))
	}
}

// 根据path匹配对应的方法
func match(path string, route Route) gin.HandlerFunc {
	return func(c *gin.Context) {
		fields := strings.Split(path, "/")
		fmt.Println("00000-fields,len(fields)=", fields, len(fields))
		if len(fields) < 3 {
			return
		}
		if len(Routes) > 0 {
			arguments := make([]reflect.Value, 1)
			arguments[0] = reflect.ValueOf(c) // *gin.Context
			route.Method.Call(arguments)
		}
	}
}
