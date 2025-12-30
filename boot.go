package caleyi

import (
	"fmt"
	"strings"

	"github.com/qinuoyun/caleyi/common"
	"github.com/qinuoyun/caleyi/utils/ci"
)

func BootStart() {

	//初始化中间件
	common.InitMiddleware()

	//初始化模型
	common.InitModule()

	//初始化服务
	common.InitServer()

	//加载路由
	r := common.InitRouter()

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
