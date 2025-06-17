package middleware

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/qinuoyun/caleyi/utils/ci"
	"regexp"
	"strconv"
	"strings"
)

// TenantVerify  验证token
func TenantVerify(c *gin.Context) {
	authStr := ci.C("tenant.auth")
	// 使用 strconv.ParseBool 将字符串转换为布尔值
	auth, err := strconv.ParseBool(authStr)
	if err != nil {
		// 处理转换失败的情况，例如打印错误信息并继续执行
		fmt.Printf("将 tenant.auth 转换为布尔值时出错: %v\n", err)
		c.Next()
		return
	}
	if auth {
		// 编译正则表达式，匹配 v1, v2, v3 等版本号格式
		versionRegex := regexp.MustCompile(`^v\d+$`)

		// 获取请求路径并去除首尾斜杠
		path := strings.Trim(c.Request.URL.Path, "/")

		// 检查是否以/api开头
		if !strings.HasPrefix(path, "api/") {
			c.Next()
			return
		}

		// 分割路径并检查段数
		segments := strings.Split(path, "/")
		if len(segments) != 5 {
			c.Next()
			return
		}

		// 检查第二段是否为版本号格式 (v1, v2, v3 等)
		if versionRegex.MatchString(segments[1]) {
			c.Next()
			return
		}

		// 满足所有条件时执行的逻辑

		// 尝试从请求头获取 tenant_id
		tenantID := c.GetHeader("tenant_id")
		if tenantID == "" {
			// 若请求头中没有，尝试从 GET 参数获取
			tenantID = c.Query("tenant_id")
		}

		// 检查是否获取到 tenant_id
		if tenantID == "" {
			c.AbortWithStatusJSON(400, gin.H{"error": "未提供 tenant_id，请通过请求头或 GET 参数传递"})
			return
		}

		// 获取 GORM 数据库实例
		db := ci.D()

		// 将 tenant_id 放入 GORM 事务上下文中
		ctx := context.WithValue(c.Request.Context(), "tenant_id", tenantID)
		db = db.WithContext(ctx)

		// 可以在这里将 tenantID 存储到上下文中，供后续处理使用
		c.Set("db", db)
		c.Set("tenant_id", tenantID)
		c.Next()
	} else {
		c.Next()
		return
	}

}
