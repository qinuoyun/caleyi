package caleyi

import (
	"fmt"
	"log"
	"strings"

	"github.com/qinuoyun/caleyi/common"
	"github.com/qinuoyun/caleyi/utils/ci"
)

func init() {
	// 框架加载时即执行软件签名授权
	if err := ci.EnsureSoftwareLicense(); err != nil {
		log.Fatalf("框架加载时软件许可证检查失败: %v", err)
	}
}

// BootStart 启动 HTTP 服务。可选：在业务包 init() 中调用 ci.BinAgentRoutes 注入 Agent API（默认前缀 /agent，见 common.bindAgentHTTPRoutes）。
func BootStart() {

	//初始化中间件
	common.InitMiddleware()

	//初始化模型
	common.InitModule()

	//初始化服务
	common.InitServer()

	//加载路由
	r := common.InitRouter()

	for _, fn := range ci.GetGinAfterRouterHooks() {
		fn(r)
	}

	routes := ""
	for _, route := range r.Routes() {
		if !strings.Contains(route.Path, "/admin/") && route.Path != "/" && !strings.Contains(route.Path, "/*filepath") {
			routes = routes + fmt.Sprintf("%v\n", route.Path)
		}
	}
	filePath := "runtime/app/routers.txt"
	err := ci.WriteToFile(filePath, routes)
	if err != nil {
		return
	}

	err = r.Run(":" + ci.C("app.app_port"))
	if err != nil {
		return
	}
}
