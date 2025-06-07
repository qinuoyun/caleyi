package caleyi

import (
	"fmt"
	"github.com/qinuoyun/caleyi/common"
	"github.com/qinuoyun/caleyi/utils/ci"
	"strings"
)

func BootStart() {

	//加载路由
	r := common.InitRouter()

	//初始化模型
	common.InitModule()

	//初始化服务
	common.InitServer()

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

	err = r.Run(":9097")
	if err != nil {
		return
	}
}
