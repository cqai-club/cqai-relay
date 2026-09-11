# Logto 统一身份与 Account Service 桥接架构

> 工作副本校验日期：2026-09-07
>
> 本文同时描述 `cqai-relay` 与 `cqai-account-service` 的职责、当前实现边界和后续验收要求，避免把“Relay 直接 OIDC 登录”和“Account Service 桥接 AI 请求”误认为同一条链路。

## 1. 目标

- Logto 是跨产品的统一身份源，负责登录、注册、用户和角色权限。
- `cqai-account-service` 是面向浏览器/下游产品的账号桥接与 AI BFF。
- `cqai-relay` 是统一 AI 网关和 NewAPI 账号承载方，负责模型、渠道、额度、Token 和 AI 协议。
- 同一个 `(issuer, subject)` 应能在不同产品中稳定定位到同一个逻辑用户；不同 `platform` 可以拥有独立的应用凭证，但不能重复创建用户。
- 浏览器不接触 NewAPI Service Token 或完整 NewAPI Key。

## 2. 两个项目的职责边界

### cqai-relay

- 管理 NewAPI 用户、API Token、额度、渠道和 AI 请求。
- 提供受内部 Token 保护的 `POST /api/internal/provision`，供 Account Service 幂等创建/复用外部身份和应用凭证。
- 当前还保留传统 Web OIDC 登录：`/oauth/oidc` 服务端换 Token，优先验证 ID Token，必要时回退读取 UserInfo，然后创建 Relay 本地 Session。

### cqai-account-service

- 校验 Logto API Access Token：签名、issuer、audience、过期时间、client ID 和必要 scope。
- 将已验证的 Logto 身份映射到固定 `platform`，不接受浏览器任意传入 platform。
- 通过 Relay 的 `/api/internal/provision` 获取或创建应用级 NewAPI Key。
- 对外提供：
  - `GET /api/account`：返回账号摘要，不返回 NewAPI Key。
  - `/v1/*`：使用服务端取得的 NewAPI Key 代理 AI 请求。

### 支付回跳的跨客户端边界

充值订单仍由 Relay 创建、保存并通过支付平台 webhook 入账。Account Service 根据 Logto Token 的 `client_id` 和客户端类型选择固定回跳：Web 按精确请求 `Origin` 选择，桌面使用 `LOGTO_CLIENT_PLATFORM_MAP` 中的自定义协议地址。调用方不再提交 `return_url`、`success_url` 或 `cancel_url`。Relay 内部 Account Service 支付入口只执行 URL 语法、凭据和危险协议检查，不依赖 `TRUSTED_REDIRECT_DOMAINS` / `TRUSTED_PAYMENT_REDIRECT_URIS`；Relay 原生管理后台/兼容入口仍执行原有白名单校验。Relay 会追加命名空间参数 `cqai_order_id`，客户端回到自身后仍需查询订单状态，不能把回跳本身当作支付成功证明。

## 3. 当前代码实际形成的两条链路

### 链路 A：Relay 管理后台登录

```text
Logto
  -> cqai-relay /oauth/oidc
  -> Relay 服务端换 Token
  -> discovery + JWKS 验证 ID Token（缺少 ID Token 时回退 /oidc/me）
  -> users.oidc_id 本地建号/复用
  -> 按已授予 role scope 同步已有用户角色
  -> Relay Session + Relay access token
  -> Relay 自身 API / AI 网关
```

### 链路 B：Account Service AI BFF

```text
Logto API Access Token
  -> cqai-account-service /api/account 或 /v1/*
  -> 校验 issuer/audience/scope/client
  -> cqai-relay /api/internal/provision
  -> external_account_identities + app_credentials
  -> 服务端使用 NewAPI Key 调用 Relay
```

链路 A 与链路 B 保持不同的认证上下文，但已在 Relay 用户层收敛：provisioning 在新建 `ExternalAccountIdentity` 前会按 `users.oidc_id == subject` 复用原生 OIDC 用户；由 Account Service 先建号时也会写入同一 `oidc_id`。Relay 前端不调用 Account Service SDK、`/api/account` 或 `/v1/*`，因为它当前定位为管理面。

## 4. 当前实现状态

| 能力 | 状态 | 说明 |
|---|---|---|
| Relay 使用 Logto 登录 | 已基本实现 | 传统 Web Authorization Code + client secret |
| 关闭 Relay 本地注册后允许 Logto 用户首次建号 | 已实现 | OIDC 被视为可信注册权威 |
| `account:admin` / `account:root` 映射 Relay 角色 | 已实现 | 首次建号写入，已有 OIDC 用户每次登录同步 |
| Relay `/api/internal/provision` | 已实现 | 内部 Token、参数校验、幂等和并发保护 |
| Account Service JWT 校验 | 已实现 | JWKS、issuer、audience、exp、scope、client map |
| Account Service `/api/account` 和 `/v1/*` | 已实现 | 服务端持有 NewAPI Key，浏览器不接触完整 Key |
| 业务 AI 请求使用 Account Service 桥接 | 已实现接口 | Relay 管理前端仍使用自身 OIDC/Session，属于独立管理面 |
| Relay OIDC 与 Account Service 复用同一本地用户 | 已实现 | 两个先后顺序都通过 `users.oidc_id == subject` 收敛 |
| 跨 platform 复用用户并分配独立凭证 | 已实现 | 同一用户不同 platform 复用用户并新建 AppCredential |
| 公网端到端闭环 | 未验证 | 本次只确认代码和测试，不代表当前运行镜像 |

## 5. 关键身份和凭证模型

### Relay 直接 OIDC 登录

- 绑定字段：`users.oidc_id`，当前主要保存 Logto `sub`。
- 账号由 `findOrCreateOAuthUser` 创建。
- 当 Token 响应包含 ID Token 时，使用 discovery 中的 issuer/JWKS 验证签名、Relay client audience、过期时间和签发时间。
- 管理角色使用 Token 响应中已授予的 Resource Scope 映射，已有 OIDC 用户会在登录时同步。
- 前端授权 URL 带 `prompt=login`，强制展示 Logto 登录页。
- 登录后使用 Relay 自身 Session 和 access token。

### Account Service 桥接

- 外部身份键：`sha256(issuer + "\\0" + subject)`。
- Relay 表：`external_account_identities`。
- 应用凭证键：`(user_id, platform)`，存储在 `app_credentials`。
- NewAPI API Key 只返回给 Account Service 服务端，不返回给 `/api/account` 或浏览器。

### 当前风险

当前单一 Logto issuer 下，两条链路已按 `subject` 复用同一 Relay 用户。剩余风险是：

- `external_account_identities` 的安全身份键包含 issuer，但 Relay 原生 `users.oidc_id` 仅保存 subject；未来多 issuer 时不能直接沿用现有收敛逻辑。
- Relay OIDC 已有用户每次登录会同步角色，Account Service provisioning 只在首次建号时使用 role，两条链路的更新语义不同。

## 6. Scope、角色和配置要求

Account Service 默认要求 `ai:invoke`；角色 scope 为：

- `account:root` -> Relay role `100`
- `account:admin` -> Relay role `10`
- 无角色 scope -> Relay role `1`

Relay 当前 OIDC 配置会请求基础 scope，并在配置了 `resource` 时追加 admin/root scope。这条链路目前定位为管理面，不直接调用 Account Service。若未来要改为直接复用 Account Service 的 Access Token，还必须保证：

- 授权请求包含 `ai:invoke`；
- `resource` 与 Account Service 的 `LOGTO_AUDIENCE` 完全一致；
- Relay 的 Logto Client ID 在 Account Service 的 `LOGTO_CLIENT_PLATFORM_MAP` 中；
- `platform` 映射固定且不接受前端覆盖。

## 7. 当前明确缺口

1. Relay 原生 OIDC 的绑定列不包含 issuer，需在接入第二个 OIDC issuer 前扩展身份模型。
2. Relay 管理面 OIDC 已在登录时同步已有用户角色，但 Account Service provisioning 只在首次建号时使用 role，两条链路的角色更新语义不一致。
3. 本地代码能力已具备，但仍需用真实 Logto 用户、公网 Account Service 和当前 Relay 运行镜像完成端到端验收。

## 8. 推荐的最终架构选择

如果目标是“Logto 统一用户 + 所有 AI 产品共享 Relay 账号和额度”，应把 Account Service 作为唯一的外部身份桥接层：

```text
Logto -> Account Service -> Relay provisioning -> Relay AI API
```

Relay 管理后台继续使用 Logto 做管理员登录，业务产品和 AI 调用统一通过 Account Service。两条链路的用户已按同一 Logto subject 收敛，但对外口径仍应区分“代码已具备”与“公网端到端已验证”。

## 9. 验收清单

- 同一 Logto 用户从两个产品进入时，Relay 只存在一个逻辑用户。
- 不同 platform 只创建不同应用凭证，不创建重复用户。
- Account Service `/api/account` 和 `/v1/*` 均能复用同一 provisioning 结果。
- Relay 关闭本地注册后，Logto 用户仍可按策略自动建号。
- `ai:invoke`、资源 audience、client-platform 映射全部一致。
- Relay 管理面登录角色同步已验证，Account Service provisioning 对已有用户的角色更新策略已明确决策。
- 浏览器网络和响应中不出现 NewAPI Service Token 或完整 NewAPI Key。
- 完成真实 Logto -> Account Service -> Relay -> AI 的端到端验证，而不只验证单元测试。
