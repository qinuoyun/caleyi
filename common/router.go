package common

//一定要导入这个Controller包，用来注册需要访问的方法
//这里路由-由构架是添加-开发者仅在指定工程目录下controller.go文件添加宝即可
import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/qinuoyun/caleyi/middleware"
	"github.com/qinuoyun/caleyi/utils/ci"
)

func InitRouter() *gin.Engine {
	//初始化路由
	R := gin.Default()
	err := R.SetTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		return nil
	}
	//访问公共目录
	R.Static("/static", "./static")

	//访问公共目录
	R.Static("/public", "./public")

	//访问公共目录
	R.Static("/uploads", "./uploads")

	//访问公共目录
	R.Static("/web", "./views/web")

	// 处理静态文件和默认页面
	R.GET("/admin/*any", func(c *gin.Context) {
		filePath := c.Param("any")
		if filePath == "" || strings.HasSuffix(filePath, "/") {
			filePath = "index.html"
		}
		fullPath := fmt.Sprintf("views/admin/%s", filePath)
		if _, err := os.Stat(fullPath); err == nil {
			c.File(fullPath)
		} else {
			c.File("views/admin/index.html")
		}
	})

	// 处理静态文件和默认页面
	R.GET("/merchant/*any", func(c *gin.Context) {
		filePath := c.Param("any")
		if filePath == "" || strings.HasSuffix(filePath, "/") {
			filePath = "index.html"
		}
		fullPath := fmt.Sprintf("views/merchant/%s", filePath)
		if _, err := os.Stat(fullPath); err == nil {
			c.File(fullPath)
		} else {
			c.File("views/merchant/index.html")
		}
	})

	//访问域名根目录重定向
	R.GET("/", func(c *gin.Context) {
		// 检查 views/web/index.html 是否存在
		if _, err := os.Stat("views/web/index.html"); err == nil {
			// 文件存在，渲染 HTML
			c.File("views/web/index.html")
		} else {
			// 文件不存在，返回 JSON
			c.JSON(200, gin.H{"code": 200, "message": "欢迎使用卡莱易框架"})
		}
	})

	R.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"X-Requested-With", "Content-Type", "Authorization", "BusinessId", "verify-encrypt", "ignoreCancelToken", "verify-time"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	//4.验证token
	R.Use(middleware.JwtVerify)

	//5.处理租户问题
	R.Use(middleware.TenantVerify)

	//6.找不到路由
	R.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method
		c.JSON(404, gin.H{"code": 404, "message": "您" + method + "请求地址：" + path + "不存在！"})
	})

	// 获取所自定义中间件
	middlewaresMap := ci.GetMiddlewares()

	// 循环遍历并判断是否有 index 方法，有则绑定
	for _, value := range middlewaresMap {
		if indexMethod, ok := value.(interface{ Index() gin.HandlerFunc }); ok {
			R.Use(indexMethod.Index())
		}
	}

	//绑定基本路由，访问路径：/User/List
	ci.Bind(R)
	//绑定插件路由
	BindSoftwareRoutes(R)
	//返回实例
	return R
}
