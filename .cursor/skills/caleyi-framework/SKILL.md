---
name: caleyi-framework
description: >-
  Guides development on the Caleyi Gin-based framework: ci package (Model, M/MT,
  tenant DB), HTTP and WebSocket routing (WsVerify, BinWSController), middleware,
  config, and team conventions from 开发规范.md. Use when working in
  github.com/qinuoyun/caleyi, adding controllers/modules, WebSocket endpoints,
  tenant or account isolation, or when the user mentions Caleyi、卡莱易、ci.M、
  BinController、WebSocket、WsVerify、BinWSController、多租户.
---

# Caleyi 框架（Gin 改造）开发 Skill

面向在本仓库或基于 Caleyi 的业务项目中写 Go 后端代码的 Agent：优先遵守仓库内《开发规范.md》，并与 `utils/ci`、`common` 的真实实现一致。

## 1. 框架定位与启动链

- **HTTP**：Gin（`github.com/gin-gonic/gin`）。
- **ORM**：GORM；全局表前缀 `pre_`、单数表名（见 `common/module.go` 的 `NamingStrategy`）。
- **典型启动**（`boot.go` 的 `BootStart`）：`InitMiddleware` → `InitModule`（连库、AutoMigrate、`ci.SetDB`）→ `InitServer` → `common.InitRouter` → `Run(":" + ci.C("app.app_port"))`。

## 2. 配置

- **`ci.C("section.key")`** 读字符串配置（`utils/ci/config.go`）。
- **加载顺序**：若存在 `config.yaml`（经 `GetConfigPath`）则优先 YAML，否则 **`config.ini`**；INI 中 `DEFAULT` 节会合并到逻辑上的 `app` 节。
- **CORS / 租户默认头**：`common.InitRouter` 中可按 `cors.*`、`app.tenant_id` 注入请求头（与规范中租户传递方式一致）。

## 3. 路由与 `/api` 组

- **`/api` 路由组**（`common.InitRouter`）：依次注册中间件 `HandleBefore` →（可选默认 `tenant_id`）→ **`middleware.JwtVerify`** → **`middleware.TenantVerify`** → `HandleAfter`。
- **应用内自动路由**（`utils/ci/AutoRouter.go`）：控制器方法映射为 **`/api` + 自 `.../app` 起的包路径片段 + 模块名 + 首字母小写方法名**（如 `Index` → `index`）。方法名前缀还决定 HTTP 方法（`Get*`/`Index` 多为 GET，其余多为 POST 等）。
- **插件/多项目路由**（`common/software.go` 的 `BindSoftwareRoutes`）：路径形如 **`/api/{AppName}/...`** 的由反射注册的控制器路由；与主应用路由并存时注意前缀与鉴权是否一致。
- **静态前端**：`frontEndProjects` 映射 `/admin`、`/h5`、`/merchant`、`/store` 等到 `./views/...`，并配合 history 回退逻辑。

### 3.1 WebSocket 路由组（`/ws`）

- **注册入口**：`common/router.go` 的 **`BindWSRoutes`**，在 `BindSoftwareRoutes` 之后执行；**不挂在 `/api` 组上**，是独立的 Gin `RouterGroup`。
- **开关与前缀**（`config.ini` / `config.yaml` 的 `[ws]` 或等价键）：
  - **`ws.enabled`**：`false` 时整组不注册（默认视为启用，只要配置了且非 `false`）。
  - **`ws.prefix`**：组前缀，默认 **`/ws`**。
  - **`ws.require_auth`**：设为 **`false`** 时跳过 JWT，仅注入 DB/租户（仅适合本地调试）；默认需鉴权。
  - **`ws.default_tenant`**：租户兜底；为空则再用 **`app.tenant_id`**。
- **鉴权中间件**：**`middleware.WsVerify`**（`middleware/WsVerify.go`）
  - 语义上对齐 **`JwtVerify` + `TenantVerify` 对 DB 的注入**：校验通过后 **`c.Set("db", db)`**，并 **`c.Set("tenant_id", ...)`**、`user` / `uid` 等与 JWT 一致，handler 里可用 **`ci.GetDB(c)`**、**`ci.M(...)`**、**`ci.GetAccountID(c)`**（与 HTTP API 同一套）。
  - **Token**：握手时浏览器往往无法自定义 Header，故支持 **`Authorization: Bearer <token>`** 或 Query **`?token=`**（`Bearer ` 前缀可带可不带）。
  - **Tenant**：优先级 **Header `tenant_id`** → **Query `tenant_id`** → **`ws.default_tenant`** → **`app.tenant_id`**。
- **业务注册方式**：在包 **`init()`** 中调用 **`ci.BinWSController(func(g *gin.RouterGroup) { ... })`**（`utils/ci/AutoSoftware.go`），在闭包里对 **`g`** 注册具体路径，例如 **`g.GET("/agent/chat", WSController{}.Chat)`**；完整 URL 为 **`{ws.prefix}{相对路径}`**（如 `/ws/agent/chat`）。
- **无处理器则静默**：`GetWSHandlersList()` 为空时不创建 WS 组。

## 4. 核心包 `utils/ci`

### 4.1 基础模型 `ci.Model`（`utils/ci/Model.go`）

- 嵌入字段：**`ID`、`CreatedAt`、`UpdatedAt`、`DeletedAt`（软删）、`TenantID`**；业务表不要重复定义这些列。
- **钩子**：`BeforeCreate` / `BeforeQuery` / `BeforeUpdate` / `BeforeDelete` 从 GORM `Statement.Context` 里取 **`tenant_id`**；缺失会报错（典型错误信息含 `tenant ID not found in context`）。
- 异步或后台任务中须保证 DB 使用的 context 带有 `tenant_id`（见下节）。

### 4.2 数据库入口

- **请求内**：`ci.M("模型名或类型")` 或 `ci.M(&Struct{})` —— 使用当前 goroutine 绑定的带租户 DB（由中间件等通过 `ci.BindDB` 注入）。
- **显式租户**：`ci.MT(tenantID, model)` 或 **`ci.DBWithTenant(tenantID)`** —— **goroutine / 异步必选**，禁止在异步里用依赖 `*gin.Context` 且未绑定的 `ci.GetDB(c)`。
- **便捷封装**：`ci.Go(c, fn)`、`ci.GoWithContext`、`ci.GoWait`、`ci.Run(c, fn)`、`ci.NewAsync(c)`（详见 `AutoModule.go` 注释）。
- **`ci.GetTenantID(c)`**：从 Gin context 取租户；异步前取出再传入。
- **`ci.GetAccountID(c)`**：从 context 的 `uid` 等解析当前账号 ID（JWT 中间件写入）。

### 4.3 HTTP 响应（`utils/ci/func.go`）

- 成功：**`ci.Success(c, data)`** → `code: 0`，`msg: "成功"`，会 **`c.Abort()`**。
- 失败：**`ci.Error(c, code, msg)`**；另有 `Message` / `Custom`。

### 4.4 注册

- **模型**：业务项目在 `init()` 里对模型实例 **`ci.BinModule(&path)`**（插件侧模型注册进 `softwareModules`，最终并入 `GetModules()`）；若使用 **`ci.RegisterModule(module, path)`** 则进入主 `modules` map（本仓库框架代码中存在该 API）。
- **控制器**：`ci.BinController(&ctrl, reflect.TypeOf(ctrl).PkgPath())`（与《开发规范》一致）；插件控制器路径需满足 `GetControllerPrefixRegex` 对 `.../controllers/...` 的约定。
- **WebSocket**：**`ci.BinWSController(fn WSHandlerFn)`**，由框架在 **`BindWSRoutes`** 里挂载到带 **`WsVerify`** 的组上（见 **§3.1**）。

## 5. 中间件要点

- **`middleware.JwtVerify`**：解析 JWT，写入用户信息；与 `ci.GetAccountID`、日志中的 `ci.GetHardwareUUID()` 等配合使用。
- **`middleware.TenantVerify`**：当 `tenant.auth` 为 true 且路径满足 `/api/...` 且段数等条件时，要求 **`tenant_id`**（Header / Query / JSON）；并把带 `tenant_id` 的 context 注入 GORM，供 `ci.Model` 钩子使用。
- **`middleware.WsVerify`**：专用于 **WS 路由组**；见 **§3.1**，与 `/api` 上的 JWT/租户链分离但 **`db`/`uid` 行为一致**。

## 6. 《开发规范.md》必须遵守的约定

### 6.1 模型（`modules/`）

- 嵌入 **`ci.Model`**；表名使用项目约定的 `TableName()`（规范示例用 `utils.GetTableName`）。
- 与 **Account** 绑定的业务表须 **`AccountID`** + 查询/创建时按规范过滤或赋值；全局字典类等可不加。

### 6.2 控制器（`controllers/`）

- 命名：**`XxxController`**。
- **导出方法名必须是单个英文单词**：`Index`、`Detail`、`Create`、`Update`、`Delete`、`All`、`Options` 等；**禁止** `GetList`、`CreateExpert` 等多词驼峰方法名（否则与自动路由规则冲突）。
- 列表/详情等账号隔离业务：**`account_id` 与 `ci.GetAccountID(c)`** 配合使用。

### 6.3 异步与租户

- 进入 `go func` 之前取出 **`tenantID := ci.GetTenantID(c)`**（及需要的 `accountID`），内部使用 **`ci.DBWithTenant` / `ci.MT` / `ci.Go`**，避免 `tenant ID not found in context`。

### 6.4 Swagger

- 按规范写 `swag` 注释；生成命令：`swag init --parseDependency --parseInternal`。

### 6.5 风格

- JSON 字段与库列名多用 **snake_case**；包名小写；错误用 `ci.Error` 或返回 `error` 视场景而定。

## 7. Agent 实施清单（新功能/改 bug）

1. 新表：定义 struct + 嵌入 `ci.Model` + `TableName` + `init` 里 **`ci.BinModule`**（或项目既有注册方式）。
2. 新接口：新建 `XxxController`，`init` 里 **`ci.BinController`**，方法名用**单词**命名。
3. 所有走 `/api` 且开启租户校验的请求：确保客户端传 **`tenant_id`**，或依赖路由里注入的默认 `app.tenant_id`。
4. 任意 goroutine：**禁止**假设仍能使用原始 `*gin.Context` 的 DB；改用 **`ci.DBWithTenant` / `ci.MT` / `ci.Go`**。
5. 响应统一 **`ci.Success` / `ci.Error`**，与现有前端约定 `code`/`msg`/`data` 一致。
6. 新增 WebSocket：在 **`init()`** 里 **`ci.BinWSController`** 注册路径；配置 **`ws.*`**；客户端握手传 **`token`**（及 **`tenant_id`**）；handler 内 **`Upgrade`** 后若开新 goroutine 读写连接，数据库仍须 **`ci.DBWithTenant(ci.GetTenantID(c))`** 或事先拷贝 `tenantID`，避免仅用已结束的 HTTP context。

## 8. 更深材料

- 完整条目与示例以仓库根目录 **`开发规范.md`** 为准；本 Skill 只保留与框架机制强相关的摘要。
- 路由与插件绑定的细节以 **`common/router.go`**（含 **`BindWSRoutes`**）、**`common/software.go`**、**`utils/ci/AutoRouter.go`**、**`middleware/WsVerify.go`**、**`utils/ci/AutoSoftware.go`（`BinWSController`）** 为准。
