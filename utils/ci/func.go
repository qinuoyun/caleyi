package ci

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// GetControllerModuleName 从控制器对象中提取模块名称
// 例如：*controllers.UserController → /user/
func GetControllerModuleName(controller interface{}) string {
	// 获取控制器类型名称（包含包路径）
	ctrlTypeName := reflect.TypeOf(controller).String()

	// 移除指针符号（如果有）
	if strings.HasPrefix(ctrlTypeName, "*") {
		ctrlTypeName = ctrlTypeName[1:]
	}

	// 提取基础类型名称（去除包路径）
	typeName := ctrlTypeName
	if dotIndex := strings.LastIndex(typeName, "."); dotIndex != -1 {
		typeName = typeName[dotIndex+1:]
	}

	// 移除 "Controller" 后缀
	if strings.HasSuffix(typeName, "Controller") {
		typeName = strings.TrimSuffix(typeName, "Controller")
	}

	// 处理特殊情况：Index → /
	if typeName == "Index" {
		return "/"
	}

	// 转换为小写并添加斜杠前缀和后缀
	return "/" + strings.ToLower(typeName) + "/"
}

// RemoveStarFromTypeName 去除类型名称中的 * 号
func RemoveStarFromTypeName(module interface{}) string {
	ctrlName := reflect.TypeOf(module).String()
	// 检查 ctrlName 是否以 * 开头，如果是则去掉 * 号
	if len(ctrlName) > 0 && ctrlName[0] == '*' {
		ctrlName = ctrlName[1:]
	}
	return ctrlName
}

// FirstUpper 字符串首字母大写
func FirstUpper(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// FirstLower 字符串首字母小写
func FirstLower(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// ValidatePhone validatePhone 验证手机号的函数
func ValidatePhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	// 简单的手机号验证正则表达式，可根据实际需求修改
	re := regexp.MustCompile(`^1[3-9]\d{9}$`)
	return re.MatchString(phone)
}

// ToInt 辅助函数，将字符串转换为整数
func ToInt(s string) int {
	var num int
	_, err := fmt.Sscanf(s, "%d", &num)
	if err != nil {
		return 0
	}
	return num
}

// I 将驼峰命名转换为蛇形命名并生成查询条件
func I(str ...string) string {
	var conditions []string
	for _, str := range str {
		if str == "ID" {
			conditions = append(conditions, "id = ?")
			continue
		}
		var result strings.Builder
		length := len(str)
		for i := 0; i < length; i++ {
			r := rune(str[i])
			if i > 0 && r >= 'A' && r <= 'Z' {
				// 检查是否为 ID
				if i+1 < length && str[i] == 'I' && str[i+1] == 'D' {
					result.WriteRune('_')
					result.WriteString("id")
					i++ // 跳过 'D'
					continue
				}
				result.WriteRune('_')
			}
			result.WriteRune(r | 0x20) // 转换为小写
		}
		condition := fmt.Sprintf("%s = ?", result.String())
		conditions = append(conditions, condition)
	}
	return strings.Join(conditions, " AND ")
}

// APIResponse 定义标准API响应结构
type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// Success 发送成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code: 0,
		Msg:  "成功",
		Data: data,
	})
	// 中断当前请求的后续处理
	c.Abort()
}

// Message 发送消息响应
func Message(c *gin.Context, Msg string) {
	c.JSON(http.StatusOK, APIResponse{
		Code: 0,
		Msg:  Msg,
	})
	// 中断当前请求的后续处理
	c.Abort()
}

// Error 发送错误响应
func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, APIResponse{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
	// 中断当前请求的后续处理
	c.Abort()
	return
}

// Custom 自定义响应内容
func Custom(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code: code,
		Msg:  msg,
		Data: data,
	})
	// 中断当前请求的后续处理
	c.Abort()
	return
}

// GetConfigPath 根据环境返回配置文件路径
func GetConfigPath(filename string) string {
	// 检查是否为开发环境
	isDev := os.Getenv("APP_ENV") == "development"

	if isDev {
		// 开发环境：使用相对路径
		return filename
	}

	// 生产环境：使用可执行文件所在目录
	exePath, err := os.Executable()
	if err != nil {
		//fmt.Printf("获取可执行文件路径失败: %v\n", err)
		return filename // 默认回退路径
	}

	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, filename)
}
