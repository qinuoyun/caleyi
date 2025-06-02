package ci

import (
	"fmt"
	"reflect"
	"strings"
)

var servers map[string]interface{}

func init() {
	servers = make(map[string]interface{})
}

// RegisterServer 注册服务
func RegisterServer(module interface{}) bool {

	vbf := reflect.ValueOf(module)
	//非模型或无方法则直接返回
	if vbf.NumMethod() == 0 {
		return false
	}
	//获取模型名称，并且去除*号的设置
	cleanedName := removeStarFromTypeName(module)

	//fmt.Printf("[%s]这里是执行了RegisterServer 注册服务\n", cleanedName)
	//存入Map列表
	servers[cleanedName] = module
	return true
}

// GetServers 用于获取所有已注册的 modules
func GetServers() map[string]interface{} {
	return servers
}

// SetServers 用于设置服务
func SetServers(name interface{}) {

}

// Server 用于调用服务
func Server(name, methodName string, args ...interface{}) (interface{}, error) {

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
	// 使用 + 操作符拼接字符串
	name = name + "Server"

	// 获取 modules 中对应的模型切片
	serverSlice, exists := servers[name]
	if !exists {
		errMsg := fmt.Sprintf("未找到对应的服务: %s", name)
		fmt.Println(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// 使用反射调用方法
	value := reflect.ValueOf(serverSlice)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	method := value.MethodByName(methodName)
	if method.IsValid() {
		methodType := method.Type()
		// 检查是否为可变参数方法
		isVariadic := methodType.IsVariadic()
		numRequiredArgs := methodType.NumIn()
		if isVariadic && numRequiredArgs > 0 {
			numRequiredArgs-- // 可变参数方法，减去可变参数本身的占位
		}

		if len(args) < numRequiredArgs {
			if numRequiredArgs == 0 {
				errMsg := fmt.Sprintf("方法 %s 不需要参数，当前传递了 %d 个参数", methodName, len(args))
				fmt.Println(errMsg)
				return nil, fmt.Errorf(errMsg)
			} else {
				errMsg := fmt.Sprintf("参数传递错误：方法 %s 需要至少 %d 个参数，实际传递了 %d 个", methodName, numRequiredArgs, len(args))
				fmt.Println(errMsg)
				return nil, fmt.Errorf(errMsg)
			}
		}

		// 将可变参数转换为 reflect.Value 切片
		var reflectArgs []reflect.Value
		for _, arg := range args {
			reflectArgs = append(reflectArgs, reflect.ValueOf(arg))
		}
		// 调用方法
		results := method.Call(reflectArgs)

		// 处理返回值
		if len(results) == 0 {
			return nil, nil
		}
		if len(results) == 1 {
			return results[0].Interface(), nil
		}
		if len(results) == 2 {
			if !results[1].IsNil() {
				return nil, results[1].Interface().(error)
			}
			return results[0].Interface(), nil
		}
		// 处理更多返回值的情况可以根据实际需求扩展
		return nil, fmt.Errorf("不支持的返回值数量: %d", len(results))
	}
	errMsg := fmt.Sprintf("未找到方法: %s", methodName)
	fmt.Println(errMsg)
	return nil, fmt.Errorf(errMsg)
}
