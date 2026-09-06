# Relay 前端接入 Logto 传统 Web 应用（管理面登录 + 业务 AI 桥接）

> 目标：让 Relay 管理面可以使用 Logto 传统 Web 登录；业务应用的 AI 调用统一走
> `Logto -> cqai-account-service -> Relay`。Relay 原生 OIDC 不在本次改造中删除，
> 只作为管理面/兼容入口保留。
>
> 现状：relay 后端已实现 Logto 传统应用所需的全部能力（服务端换 token 的
> `oauth/oidc.go`、建号/绑定 `controller/oauth.go`、`/api/internal/provision`、
> `AuthFlow` 状态/会话）。本文档只做「校正配置 + 前端对齐 + 角色路由」三类小改，
> 不引入 SPA SDK、不做跨域代理改造。

## 0. 决策记录（仅用于理解，不改代码）

- relay 前端是一个 **传统 Web 应用**（Go 后端同源托管页面），因此用
  **Authorization Code + client_secret** 在**服务端**换 token，而不是 SPA 的 PKCE。
- 登录 token 存 **sessionStorage**（仅本标签页有效，关标签即失效，避免 XSS 泄漏；
  不落 localStorage）。
- 角色控制：Logto `scope` 中的 `account:root` → NewAPI `role=100`，
  `account:admin` → `role=10`，无 scope → `role=1`（普通用户）。
- 首次登录且系统关闭注册（`RegisterEnabled=false`）时：Logto 是 OIDC 用户的
  注册权威，所有成功认证的 Logto 用户都允许自动创建 Relay 本地镜像账号；
  `account:admin`/`account:root` 只负责映射 Relay 角色。`RegisterEnabled` 仍然
  控制本地用户名密码注册以及其他 OAuth 提供商的自动建号。

---

## 1. 后端（Go，全部在 `cqai-relay`）

### 1.1 `setting/system_setting/oidc.go` — 补充配置字段

`OIDCSettings` 结构体新增字段（保持与数据库 `options` 表已有键兼容，
新字段通过 config 注册自动持久化）：

```go
type OIDCSettings struct {
    Enabled               bool   `json:"enabled"`
    DisplayName           string `json:"display_name"`
    ClientId              string `json:"client_id"`
    ClientSecret          string `json:"client_secret"`
    WellKnown             string `json:"well_known"`
    AuthorizationEndpoint string `json:"authorization_endpoint"`
    TokenEndpoint         string `json:"token_endpoint"`
    UserInfoEndpoint      string `json:"user_info_endpoint"`
    Resource              string `json:"resource"`       // Logto API Resource indicator
    // 新增：
    RedirectURI     string `json:"redirect_uri"`      // 例如 http://localhost:3001/oauth/oidc
    Scope           string `json:"scope"`              // 默认 "openid profile email"
    AdminScope      string `json:"admin_scope"`        // 默认 "account:admin"
    RootScope       string `json:"root_scope"`         // 默认 "account:root"
}
```

默认值放在 `var defaultOIDCSettings = OIDCSettings{ Scope: "openid profile email", AdminScope: "account:admin", RootScope: "account:root" }`。
`GetEffectiveDisplayName()` 保持现状。

### 1.2 `oauth/oidc.go` — Logto 对齐 + 角色映射

`ExchangeToken`：

- `redirectUri` 改用 `settings.RedirectURI`（为空时回退到
  `system_setting.ServerAddress + "/oauth/oidc"`），不要硬拼。
- `scope` 参数改为使用 `settings.Scope`。
- 当配置了 `settings.Resource` 时，Relay 会自动把 `admin_scope` 和
  `root_scope` 追加到授权请求和 token 请求，避免角色账号因遗漏资源 scope
  而被当作普通用户。
- `resource` 参数使用 Logto API Resource 的 **API Identifier**，例如
  `https://account.cqaiclub.asia`。Logto 的资源权限不会出现在 OIDC 基础
  discovery 的 `scopes_supported` 中，必须显式带 resource indicator。
- 请求增加可选 `code_verifier`（若以后要支持 PKCE；本次可传空）。

`GetUserInfo`：

- 优先校验并读取 Logto 返回的 **ID Token**，不再把带有 API Resource
  audience 的 Access Token 发送到 `/oidc/me`。ID Token 使用 discovery 文档中的
  `issuer`、`jwks_uri` 和客户端 `client_id` 校验签名、发行者、受众、过期时间和
  签发时间。
- ID Token 只读取标准用户字段 `sub`、`email`、`name`、`preferred_username` 和
  `username`。Relay 管理员角色使用 Token 响应中的已授予 Resource Scope 映射，
  不依赖 Logto 自定义 JWT Claims。
- 只有在 Token 响应没有 ID Token 时，才回退调用 `/oidc/me`；因此配置了
  `resource=https://account.cqaiclub.asia` 时，Relay 原生登录仍然可以使用资源权限，
  同时不会把 Account Service 的资源 Token 当作 UserInfo Token。
- 已存在的 OIDC 用户每次登录都会根据已授予的角色 scope 同步本地角色，避免首次
  登录时的普通用户角色永久保留。

### 1.3 `controller/oauth.go` — 注册开关 + 角色写入

`findOrCreateOAuthUser`：

- 在 `if !common.RegisterEnabled` 分支之前，允许可信的 OIDC 提供商自动建号；
  普通 OIDC 用户创建为 `role=1`，admin/root scope 继续映射为对应 Relay 角色。
- 创建用户后写角色：`user.Role = role`（role>0 时）。

### 1.4 `controller/misc.go` — status 暴露前端需要的字段

在 `GetStatus`（`controller/misc.go` 第 113-116 行附近）的 OIDC 段新增：

```go
"oidc_redirect_uri":           system_setting.GetOIDCSettings().RedirectURI,
"oidc_scope":                  system_setting.GetOIDCSettings().Scope,
```

（`oidc_enabled` / `oidc_client_id` / `oidc_authorization_endpoint` /
`oidc_display_name` 已存在，保留。）

### 1.5 验证

```bash
cd cqai-relay && gofmt -w setting/system_setting/oidc.go oauth/oidc.go controller/oauth.go controller/misc.go && go build ./... && cd relaykit && GOWORK=off go build ./...
```

---

## 2. 前端（React，`cqai-relay/web`）

### 2.1 `web/src/features/auth/types.ts` — SystemStatus 加字段

`SystemStatus` 接口新增：

```ts
oidc_redirect_uri?: string
oidc_resource?: string
oidc_scope?: string
```

### 2.2 `web/src/features/auth/lib/oauth.ts` — 构建 Logto 授权 URL

`buildOIDCOAuthUrl` 增加可选 `scope` 参数：

```ts
export function buildOIDCOAuthUrl(
  authUrl: string,
  clientId: string,
  state: string,
  redirectUri?: string,
  scope?: string
): string {
  const url = new URL(authUrl)
  url.searchParams.set('client_id', clientId)
  url.searchParams.set('redirect_uri', redirectUri || `${window.location.origin}/oauth/oidc`)
  url.searchParams.set('response_type', 'code')
  if (resource) url.searchParams.set('resource', resource)
  url.searchParams.set('scope', scope || 'openid profile email')
  url.searchParams.set('state', state)
  return url.toString()
}
```

### 2.3 `web/src/features/auth/hooks/use-oauth-login.ts` — 传 scope + redirect_uri

`handleOIDCLogin` 内，把 `status.oidc_scope` / `status.oidc_redirect_uri` /
`status.oidc_resource` 传
给 `buildOIDCOAuthUrl`（现实现只传了 endpoint/clientId/state，未传 scope
和 redirect_uri，导致 Logto 收不到 `account:admin/root` scope）。

### 2.4 `web/src/routes/_authenticated/route.tsx` — 按角色展示路由

`beforeLoad` 校验登录后，从 `auth.user.role` 判断：

- `role === 100`（root）→ 允许全部路由（含管理后台 `/admin/*`）。
- `role === 10`（admin）→ 允许管理相关路由。
- `role === 1`（common）→ 只允许用户自身页面（`/dashboard`、`/profile`、
  `/user/*`、`/token/*` 等），**重定向到 `/dashboard` 并隐藏侧边栏管理入口**。
- 未登录一律先跳 Logto 授权（现有逻辑已做，保留）。

需要在路由文件内新增一个 `allowedRoles` 判断函数（如 `canAccessRoute(role, path)`），
对不满足的路由 `throw redirect({ to: '/dashboard' })`。

### 2.5 `web/src/features/auth/components/oauth-providers.tsx` — 保留 OIDC 按钮

保留现有 `status?.oidc_enabled` 分支（按钮显示名来自 `oidc_display_name`），
只确保它走更新后的 `handleOIDCLogin`（自动带 scope/redirect_uri）。

### 2.6 验证

```bash
cd cqai-relay/web && bun install && bun run build
```

---

## 3. 业务应用 → Account Service 调用（统一业务链路）

业务应用需要拿 `GET /api/account`（用户 quota）或代理 `/v1/*` 时，
必须复用 `cqai-account-service` 已提供的两个能力，不直接把 Relay Key 暴露给浏览器：

- `GET /api/account`：带 `Authorization: Bearer <Logto access token>`，
  Account Service 在服务端用它换 NewAPI 应用 Key 并返回 `{userId, platform, quota, quotaUsed}`。
- `/v1/*`：Account Service 代理到 NewAPI。
- relay 前端只需把 Logto token 存 sessionStorage，并在 axios 实例里加
  `Authorization: Bearer`；CORS 白名单在 Account Service 的
  `CORS_ALLOWED_ORIGINS`（本地联调需含 `http://localhost:3001`）。

> 注意：`/api/account` 不是登录接口，不建 Relay 会话。Relay 管理后台仍可由自己的
> `setupLogin`（cookie/session）维护；业务 AI 权限由 Logto Access Token 和
> Account Service 校验。

---

## 4. 部署与配置清单

### 4.1 Logto 应用（传统 Web 应用）

- App type: Traditional Web (Confidential)
- Redirect URI: `http://localhost:3001/oauth/oidc`（开发）/
  `https://relay.cqaiclub.asia/oauth/oidc`（生产）
- 授权方式：Authorization Code + `client_secret`
- 批准 scope：`openid profile email` **以及 `account:admin`、`account:root`**
  （后两个由角色绑定到具体用户）
- API Resource：`https://account.cqaiclub.asia`，并把 `account:admin` /
  `account:root` 权限分配给对应 Logto 角色。

### 4.2 relay 后台系统设置（`options` 表）

| Key | 值 |
|---|---|
| `oidc.enabled` | `true` |
| `oidc.client_id` | Logto 传统应用 Client ID |
| `oidc.client_secret` | Logto 传统应用 Client Secret（加密存储） |
| `oidc.authorization_endpoint` | `https://auth.cqaiclub.asia/oidc/auth` |
| `oidc.token_endpoint` | `https://auth.cqaiclub.asia/oidc/token` |
| `oidc.user_info_endpoint` | `https://auth.cqaiclub.asia/oidc/me` |
| `oidc.well_known` | `https://auth.cqaiclub.asia/oidc/.well-known/openid-configuration` |
| `oidc.redirect_uri` | `http://localhost:3001/oauth/oidc` |
| `oidc.resource` | `https://account.cqaiclub.asia` |
| `oidc.scope` | `openid profile email`（配置 resource 后自动追加角色 scope） |
| `oidc.admin_scope` | `account:admin` |
| `oidc.root_scope` | `account:root` |
| `ServerAddress` | `http://localhost:3001`（后端拼接回跳用，需与前端一致） |

### 4.3 本地联调

1. 启动 relay：`docker compose up -d`（或本地跑 `new-api`，端口 3000）。
2. 启动 account-service：`cd cqai-account-service && npm run dev`
   （读取 `apps/server/.env`，`PORT=8789`、`CORS_ALLOWED_ORIGINS` 需含
   `http://localhost:3001`）。
3. 打开 `http://localhost:3001` → 应看到 OIDC 登录按钮 → 跳 Logto →
   回跳后进入 `/dashboard`；admin/root 账号能看到管理后台，普通用户只看到用户页。

---

## 5. 验收标准

- [ ] `localhost:3001` 直接访问 → 未登录跳 Logto，登录成功进入 dashboard。
- [ ] Logto 中带 `account:root` scope 的账号登录后 `role=100`，可见完整管理后台。
- [ ] Logto 中带 `account:admin` scope 的账号登录后 `role=10`，可见管理页面。
- [ ] 无 scope 的账号登录后 `role=1`，只可见用户页，访问 `/admin/*` 被重定向。
- [ ] 关闭 `RegisterEnabled` 后：Logto 普通用户、admin 用户和 root 用户都能自动建号；
      其中 admin/root scope 分别映射到 Relay 管理员/root 角色。
- [ ] 后端 `go build ./...` 与前端 `bun run build` 全绿。
- [ ] token 存 sessionStorage，关标签页后重新访问需重新登录。

## 6. 相关文件速查

| 文件 | 改动 |
|---|---|
| `setting/system_setting/oidc.go` | 新增 RedirectURI/Scope/AdminScope/RootScope + 默认值 |
| `oauth/oidc.go` | redirect_uri/scope 用配置；校验 ID Token 并按已授予 scope 映射角色 |
| `controller/oauth.go` | Logto 用户可自动建号；admin/root scope 映射角色；本地注册开关仍限制其他提供商 |
| `controller/misc.go` | status 暴露 oidc_redirect_uri/oidc_scope |
| `web/src/features/auth/types.ts` | SystemStatus 加 oidc_redirect_uri/oidc_scope |
| `web/src/features/auth/lib/oauth.ts` | buildOIDCOAuthUrl 支持 scope/redirectUri |
| `web/src/features/auth/hooks/use-oauth-login.ts` | 登录传递 scope/redirect_uri |
| `web/src/routes/_authenticated/route.tsx` | 按角色路由守卫 |
