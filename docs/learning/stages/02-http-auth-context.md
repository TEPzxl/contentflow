# Stage 2: HTTP 路由、认证和请求上下文

## 0. 术语预解释

- 路由：HTTP 方法和 URL 到具体 handler 函数的映射。
- Handler：处理一次 HTTP 请求的函数或对象。
- Token：客户端证明自己身份的一段字符串。
- JWT：一种带签名的 token 格式，服务端可以验证它是否被篡改。
- Request context：一次请求携带的上下文数据，业务代码可以从中读取 userID。

## 1. Learning Target

你需要能解释登录如何生成 token，以及后续请求如何拿到 userID。

## 2. Files to Read

- `internal/http/router.go`
- `internal/http/middleware/auth.go`
- `internal/http/requestctx/context.go`
- `internal/module/auth/route.go`
- `internal/module/auth/handler.go`
- `internal/module/auth/service.go`
- `internal/module/auth/token.go`
- `internal/module/user/repository.go`

## 3. Reading Sequence

1. `router.go` 看总路由。
2. `auth/route.go` 看认证 API。
3. `auth/handler.go` 看 HTTP request/response。
4. `auth/service.go` 看业务规则。
5. `auth/token.go` 看 token 生成和解析。
6. `middleware/auth.go` 看 userID 写入 context。

## 4. Key Code Objects

| Name | Type | File | Why It Matters |
|---|---|---|---|
| `NewRouter` | function | `internal/http/router.go` | 所有 route 和 middleware 的入口 |
| `AuthRequired` | function | `internal/http/middleware/auth.go` | 保护需要登录的 API |
| `AuthService` | struct | `internal/module/auth/service.go` | 注册、登录、刷新、登出核心逻辑 |
| `JWTTokenManager` | struct | `internal/module/auth/token.go` | access token 和 refresh token 管理 |
| `RefreshTokenRepository` | interface | `internal/module/auth/refresh_token_repository.go` | refresh token 持久化契约 |

## 5. Hands-On Checks

```fish
go test ./internal/http/... ./internal/module/auth/...
```

## 6. Source Notes

- 登录成功返回 access token，并设置 refresh cookie。
- refresh token 原文不存数据库，只存 SHA-256 hash。
- 受保护请求要求 `Authorization: Bearer <token>`。
- `AuthRequired` 解析 access token 后把 userID 写进 request context。
- `/me` 不查请求 body，只从 context 拿 userID。

## 7. Diagram Task

画出：

```text
POST /api/v1/auth/login
  -> Handler.Login
  -> AuthService.Login
  -> userRepo.FindByEmail
  -> verifyPassword
  -> GenerateAccessToken / GenerateRefreshToken
  -> refreshTokenRepo.Create
```

再画出：

```text
GET /api/v1/me
  -> AuthRequired
  -> ParseAccessToken
  -> requestctx.WithUserID
  -> Handler.Me
```

## 8. Self-Test

1. 登录失败为什么统一返回 `invalid_credentials`？
2. refresh token 为什么要旋转？
3. access token 过期时间在哪里配置？
4. middleware 为什么不直接查数据库？
5. `requestctx.UserID` 返回 false 时 handler 怎么处理？
6. cookie 的 path 为什么是 `/api/v1/auth`？
7. 用户注册时如何避免重复邮箱？
8. 哪些接口不需要登录？

## 9. Interview Drill

### 30-second explanation

“登录时 service 查用户、校验 bcrypt 密码，生成 JWT access token 和随机 refresh token。数据库只保存 refresh token hash。后续请求通过 Bearer token 进入 auth middleware，解析出 userID 后写入 request context。”

### 2-minute explanation

“认证模块分层很清楚。handler 负责绑定 JSON 和 HTTP 错误码，service 负责邮箱规范化、密码强度、密码 hash、token 生命周期，repository 负责用户和 refresh token 持久化。refresh 流程会撤销旧 token 并创建新 token，降低 refresh token 被重复使用的风险。”

### Follow-up questions

| Question | Answer Outline |
|---|---|
| access token 和 refresh token 区别？ | 前者短期访问，后者换取新 access token |
| 为什么存 hash？ | 降低数据库泄露风险 |
| userID 如何传递？ | middleware 写入 request context |
| 认证模块有什么不足？ | 无角色权限、无组织模型 |
| refresh token 无效返回什么？ | 401 `invalid_refresh_token` |

## 10. Resume Risk Notes

可以说“实现了 JWT 登录和 refresh token 旋转”，不要说“完整权限系统”。

## 11. Completion Checklist

- [ ] 能画出登录链路。
- [ ] 能解释 request context。
- [ ] 能说明 refresh token hash 存储。
- [ ] 能指出认证边界不足。
