package ci

import (
	"reflect"
)

var middlewares map[string]interface{}

func init() {
	middlewares = make(map[string]interface{})
}

// RegisterMiddlewares 注册服务
func RegisterMiddlewares(value interface{}) bool {

	vbf := reflect.ValueOf(value)
	//非中间件或无方法则直接返回
	if vbf.NumMethod() == 0 {
		return false
	}
	//获取中间件名称，并且去除*号的设置
	cleanedName := RemoveStarFromTypeName(value)

	//fmt.Printf("[%s]RegisterMiddlewares 注册中间件\n", cleanedName)
	//存入Map列表
	middlewares[cleanedName] = value
	return true
}

// GetMiddlewares 用于获取所有已注册的 modules
func GetMiddlewares() map[string]interface{} {
	return middlewares
}
