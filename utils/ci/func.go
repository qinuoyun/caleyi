package ci

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"regexp"
	"strings"
)

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
