package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qinuoyun/caleyi/utils/ci"
)

// WsVerify WS 专属认证中间件
//
// 功能等价于 /api 路由上的 JwtVerify + TenantVerify，但：
//   - 支持 Query 参数传 token（WebSocket 握手时浏览器无法自定义 Header）
//   - 不校验路径白名单（WS 路由全部需要鉴权，若关闭鉴权用 ws.require_auth=false）
//
// token 解析优先级：
//  1. Header  Authorization: Bearer <token>
//  2. Query   ?token=<token>
//
// tenant_id 解析优先级：
//  1. Header  tenant_id
//  2. Query   ?tenant_id=
//  3. Config  ws.default_tenant
//  4. Config  app.tenant_id
func WsVerify(c *gin.Context) {
	if ci.C("ws.require_auth") == "false" {
		wsInjectDB(c, wsResolveTenantID(c))
		c.Next()
		return
	}

	// ── 提取 token ──────────────────────────────────────────────────────────────
	token := c.GetHeader("Authorization")
	if token == "" {
		token = c.GetHeader("authorization")
	}
	if token == "" {
		token = c.Query("token")
	}
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	if token == "" {
		c.JSON(401, gin.H{"code": 401, "msg": "token 不存在"})
		c.Abort()
		return
	}

	// ── 校验 token ──────────────────────────────────────────────────────────────
	claims, err := ParseToken(token)
	if err != nil {
		c.JSON(401, gin.H{"code": 401, "msg": "token 无效: " + err.Error()})
		c.Abort()
		return
	}

	c.Set("user", claims.UserClaims)
	c.Set("uid", claims.UserClaims.ID)
	if claims.UserClaims.Module == "" {
		c.Set("user_module", "user")
	} else {
		c.Set("user_module", claims.UserClaims.Module)
	}

	// ── 注入 DB（携带 tenant_id 的 GORM 实例） ────────────────────────────────
	wsInjectDB(c, wsResolveTenantID(c))
	c.Next()
}

// wsResolveTenantID 按优先级解析 tenant_id
func wsResolveTenantID(c *gin.Context) string {
	if v := c.GetHeader("tenant_id"); v != "" {
		return v
	}
	if v := c.Query("tenant_id"); v != "" {
		return v
	}
	if v := ci.C("ws.default_tenant"); v != "" {
		return v
	}
	return ci.C("app.tenant_id")
}

// wsInjectDB 将携带 tenant_id 的 DB 实例注入 gin 上下文
// 效果等同于 TenantVerify 对 /api 路由所做的操作
func wsInjectDB(c *gin.Context, tenantID string) {
	db := ci.D()
	if tenantID != "" {
		ctx := context.WithValue(c.Request.Context(), "tenant_id", tenantID)
		db = db.WithContext(ctx)
		c.Set("tenant_id", tenantID)
	}
	c.Set("db", db)
}
