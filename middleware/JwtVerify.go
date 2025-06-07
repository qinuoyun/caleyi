package middleware

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/qinuoyun/caleyi/utils/ci"
)

// UserClaims 用户信息类，作为生成token的声明
type UserClaims struct {
	ID         int64  `json:"id"`
	AccountId  int64  `json:"accountId"`  // A端主账号id
	BusinessID int64  `json:"businessID"` // B端主账号id
	Openid     string `json:"openid"`     // 微信openid
	Name       string `json:"name"`
	Username   string `json:"username"`
}

// CustomClaims 自定义声明结构体，嵌入标准声明
type CustomClaims struct {
	UserClaims
	jwt.RegisteredClaims // 包含标准声明（过期时间、签发时间等）
}

var (
	// 自定义的token秘钥
	secret = []byte("16849841325189456f489")
	// EffectTime token有效时间（通过配置加载）
	EffectTime time.Duration
)

// 初始化函数，加载配置
func init() {
	EffectTime = time.Duration(getJwtInt()) * time.Minute // 分钟单位
}

// getJwtInt 从配置获取JWT过期时间（默认2小时）
func getJwtInt() int64 {
	// 加载配置
	num := "72000"
	intNum, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 2 * 60 // 默认2个小时（分钟）
	}
	return intNum
}

// TokenOutTime 返回超时时间
func TokenOutTime(claims *UserClaims) int64 {
	return time.Now().Add(EffectTime).Unix()
}

// GenerateToken 生成token
func GenerateToken(claims *UserClaims) (string, error) {
	// 设置自定义声明和标准声明
	customClaims := CustomClaims{
		UserClaims: *claims,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(EffectTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   strconv.FormatInt(claims.ID, 10),
		},
	}

	// 生成token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, customClaims)
	signedToken, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

// JwtVerify 验证token
func JwtVerify(c *gin.Context) {
	// 获取白名单列表
	whitelistItems := ci.C("whitelist.items")
	// 转换列表数据
	whiteList := strings.Split(whitelistItems, ",")
	if checkWhiteList(whiteList, c.Request.URL.Path) { // 不需要token验证的路径
		return
	}

	// 从请求头获取token
	token := c.GetHeader("Authorization")
	if token == "" {
		token = c.GetHeader("authorization")
	}
	if token == "" {
		c.JSON(401, gin.H{
			"code": 401,
			"msg":  "token 不存在",
		})
		c.Abort()
		return
	}

	// 分割字符串，提取Bearer Token
	parts := strings.SplitN(token, " ", 2)
	if !(len(parts) == 2 && parts[0] == "Bearer") {
		c.JSON(401, gin.H{
			"code": 401,
			"msg":  "Authorization header格式必须为 Bearer [token]",
		})
		c.Abort()
		return
	}
	token = parts[1]

	// 验证token并解析声明
	claims, err := ParseToken(token)
	if err != nil {
		c.JSON(401, gin.H{
			"code": 401,
			"msg":  "token无效: " + err.Error(),
		})
		c.Abort()
		return
	}

	// 将用户信息存储在请求上下文中
	c.Set("user", claims.UserClaims)
	c.Set("user_id", claims.UserClaims.ID)
}

// ParseToken 解析Token
func ParseToken(tokenString string) (*CustomClaims, error) {
	// 定义声明结构体
	claims := &CustomClaims{}

	// 解析token
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		// 区分不同错误类型
		if err == jwt.ErrSignatureInvalid {
			return nil, fmt.Errorf("签名无效")
		}
		return nil, err
	}

	return claims, nil
}

// Refresh 更新token
func Refresh(tokenString string) (string, error) {
	// 解析旧token
	claims, err := ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	// 更新过期时间
	claims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(EffectTime))

	// 生成新token
	return GenerateToken(&claims.UserClaims)
}

// 检查白名单
func checkWhiteList(whiteList []string, path string) bool {
	for _, p := range whiteList {
		if strings.HasPrefix(p, "^") {
			matched, _ := regexp.MatchString(p, path)
			if matched {
				return true
			}
		} else {
			if p == path {
				return true
			}
		}
	}
	return false
}
