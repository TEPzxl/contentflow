# Stage 3: Source 管理和数据库边界

## 0. 术语预解释

- Source：内容来源，项目中可以是 RSS feed 或 Email mailbox。
- CRUD：创建、读取、更新、删除四类基础操作。
- 软删除：不真正删除数据库行，而是写入删除时间。
- 脱敏：把密码、token、密钥等敏感字段替换成不可见值。
- SSRF：服务端被诱导访问本不该访问的内部地址或本地地址的安全风险。

## 1. Learning Target

你需要能解释 source 如何创建、校验、按用户隔离、缓存和软删除。

## 2. Files to Read

- `migrations/000001_create_initial_tables.up.sql`
- `internal/module/source/type.go`
- `internal/module/source/model.go`
- `internal/module/source/route.go`
- `internal/module/source/handler.go`
- `internal/module/source/service.go`
- `internal/module/source/repository.go`
- `internal/module/source/cache.go`
- `internal/netguard/netguard.go`

## 3. Reading Sequence

1. 先看 `sources` 表。
2. 看 `TypeRSS` 和 `TypeEmail`。
3. 看 `Source` model。
4. 看 route 和 handler。
5. 看 service 的 normalize 和 redact。
6. 看 repository 的 user scope。
7. 看 netguard。

## 4. Key Code Objects

| Name | Type | File | Why It Matters |
|---|---|---|---|
| `Source` | struct | `source/model.go` | source 表映射 |
| `SourceService` | struct | `source/service.go` | source 业务规则 |
| `normalizeURL` | function | `source/service.go` | URL 格式和安全校验 |
| `redactConfig` | function | `source/service.go` | 防敏感配置泄露 |
| `FindByUserIDAndID` | method | `source/repository.go` | 用户隔离关键 |
| `RedisListCache` | struct | `source/cache.go` | source 列表缓存 |

## 5. Hands-On Checks

```fish
go test ./internal/module/source/... ./internal/netguard
```

## 6. Source Notes

- source type 只有 `rss` 和 `email`。
- RSS source 创建时必须有 HTTP/HTTPS URL。
- URL 会经过 `netguard.ValidateHTTPURL`，阻止 localhost、private IP、link-local 等地址。
- repository 查询始终带 `user_id` 和 `deleted_at IS NULL`。
- 删除是 soft delete，会把 `is_active` 改成 false。
- 返回 source 时会对 password、token、api_key 等字段脱敏。

## 7. Diagram Task

```text
POST /sources
  -> AuthRequired
  -> Handler.Create
  -> SourceService.CreateSource
  -> normalizeName / normalizeURL / normalizeConfig
  -> Repository.Create
  -> DeleteUser cache
```

## 8. Self-Test

1. `sources` 表为什么有 `deleted_at`？
2. `idx_sources_user_url_active` 解决什么问题？
3. 为什么 source URL 不能允许 localhost？
4. Email source 是否必须有 URL？
5. source list cache key 包含哪些字段？
6. 更新 source 后为什么要清 cache？
7. config 里哪些 key 会被脱敏？
8. `ErrSourceNotFound` 为什么会转换成 `ErrSourceNotAccessible`？

## 9. Interview Drill

### 30-second explanation

“source 模块负责 RSS 和 Email 来源管理。创建时会校验 name、type、URL 和 config；查询、更新、删除都按 userID 过滤；删除用 soft delete；返回配置前会对敏感字段脱敏。”

### 2-minute explanation

“source 是采集链路的起点。RSS source 需要 URL，并且 URL 会经过 netguard 校验，避免服务端访问本地或私有地址。repository 层用 userID 和 sourceID 共同查询，确保用户只能访问自己的 source。列表结果可以缓存在 Redis，写操作后按用户删除缓存。”

### Follow-up questions

| Question | Answer Outline |
|---|---|
| 为什么要 soft delete？ | 保留历史关系和避免误删 |
| 如何防重复 source？ | user + url + active unique index |
| 如何防敏感信息返回前端？ | `redactConfig` 递归处理敏感 key |
| netguard 防什么？ | SSRF 风险 |
| source 和 collector 如何连接？ | collector service 按 source type 从 registry 找 collector |

## 10. Resume Risk Notes

可以说“实现了 source 的用户隔离、URL 安全校验和敏感配置脱敏”，不要说“完整数据权限系统”。

## 11. Completion Checklist

- [ ] 能解释 source 表字段。
- [ ] 能解释 URL 安全校验。
- [ ] 能解释 source user scope。
- [ ] 能解释 cache invalidation。

