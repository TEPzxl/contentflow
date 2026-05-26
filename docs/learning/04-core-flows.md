# 核心流程拆解

## 0. 术语预解释

- 流程：一次功能从入口到输出的完整执行路径。
- 调用链：一个函数调用另一个函数形成的顺序。
- 中间件：HTTP 请求到达业务 handler 前后自动执行的处理逻辑。
- 持久化：把数据写入 PostgreSQL 这样的数据库。
- 缓存：把高频读取结果临时放到 Redis，减少数据库查询。
- 锁：防止同一个 source 被多个采集任务同时处理的机制。
- 重试：失败后按规则再次执行。
- 死信队列：多次失败后把任务保存到单独位置，等待人工处理或 replay。
- 幂等：同一个请求重复执行时，结果不会产生不可接受的重复副作用。

## Flow: 应用启动和路由装配

### 1. Why This Flow Matters

它决定项目从命令行启动后会创建哪些依赖、暴露哪些 API、启动哪些后台任务。第一阶段学习必须先理解这条链路。

### 2. Entry Point

- File: `cmd/server/main.go`
- Function: `main()`
- Command: `go run ./cmd/server` 或 `make run`
- Trigger: 启动后端进程

### 3. Call Sequence

1. `cmd/server/main.go/main`
2. `internal/app/server.go/Run`
3. `internal/config/config.go/Load`
4. `internal/app/mode.go/runtimePlanForMode`
5. `internal/database/postgres.go/NewPostgres`
6. `internal/cache/redis.go/NewRedis`
7. `internal/app/server.go` 创建 repositories、services、handlers、scheduler、worker
8. `internal/http/router.go/NewRouter`
9. `http.Server.ListenAndServe`

### 4. Input and Output

| Step | Input | Output |
|---|---|---|
| `config.Load` | `CONTENTFLOW_CONFIG` 或默认 `configs/config.yaml` | `config.Config` |
| `runtimePlanForMode` | `app.mode` | 是否启动 HTTP / scheduler / worker |
| `NewPostgres` | database config | `*gorm.DB` |
| `NewRedis` | redis config | `*redis.Client` |
| `NewRouter` | logger、db、redis、route registration | `*gin.Engine` |
| `ListenAndServe` | `server.host:server.port` | HTTP API 对外服务 |

### 5. Data Structures

- `config.Config`：聚合 app、server、database、redis、kafka、auth、rate limit、cache、observability 配置。
- `runtimePlan`：决定 HTTP、Scheduler、Worker 是否启动。

### 6. Error Handling

- 启动期错误会逐层 wrap 后返回给 `main()`。
- `main()` 记录 `run server failed` 并 `os.Exit(1)`。
- HTTP server 关闭时忽略 `http.ErrServerClosed`，正常信号退出时执行 shutdown。

### 7. Persistence

启动本身不创建表；表由 migration 负责。启动时会连接 PostgreSQL 并 ping。

### 8. Cache / Lock / Rate Limit

启动时创建 Redis client，并在 app 组装时创建：

- source list cache：`cache:sources`
- article list cache：`cache:articles`
- collection lock：`collection:lock:source:<sourceID>`
- login rate limit：`ratelimit:login:ip:<ip>`
- collect rate limit：`ratelimit:collect:user:<userID>:id:<sourceID>`

### 9. Async / Queue / Retry / Idempotency

如果 `kafka.enabled=true`：

- 创建 `KafkaWriter`、`KafkaReader`。
- 创建 `OutboxProducer` 和 `OutboxDispatcher`。
- API 的 collect route 改为 async handler。
- worker 模式会消费 `collection.requested`。

### 10. Observability

- `observability.InitTracing` 根据配置启用 tracing。
- `observability.NewDefaultMetrics` 注册 Prometheus metrics。
- `NewRouter` 挂载 `/metrics`。
- `Metrics.RegisterGormCallbacks` 记录 GORM 操作指标。

### 11. Important Edge Cases

- `app.mode=worker` 但 `kafka.enabled=false` 会返回错误。
- `app.mode` 非法会返回 unsupported mode。
- 生产环境弱 JWT secret 会被 `validateSecurity` 拦截。

### 12. 30-Second Interview Explanation

后端入口是 `cmd/server/main.go`，它只调用 `app.Run()`。`Run()` 先加载配置，再根据 `app.mode` 决定启动 API、scheduler、worker 或全部；随后初始化 PostgreSQL、Redis、认证、业务 service、Kafka outbox/worker、metrics/tracing，最后通过 `http.NewRouter` 注册路由并启动 HTTP server。

### 13. 2-Minute Interview Explanation

这个项目把启动装配集中在 `internal/app/server.go`。配置由 Viper 读取，并支持 `CONTENTFLOW_` 环境变量覆盖。启动后会先建立数据库和 Redis 连接，再创建用户、认证、来源、文章、采集、AI 等 repository/service/handler。Kafka 开关会改变采集路由：关闭时直接调用 collector service，开启时先写 outbox，dispatcher 再投递 Kafka，worker 异步执行。运行模式由 `app.mode` 控制，Kubernetes 中 backend、worker、scheduler 分别通过环境变量指定不同模式。

### 14. Likely Interview Questions

| Question | Answer Outline |
|---|---|
| 为什么需要 `app.mode`？ | 让同一份二进制按 API、scheduler、worker 拆开部署 |
| Kafka 关闭时采集如何执行？ | HTTP collect route 直接调用 `CollectionService.CollectSource` |
| Kafka 开启时 route 有什么变化？ | collect route 使用 `AsyncHandler.RequestCollection`，写 outbox 返回 queued |
| 配置如何覆盖？ | Viper 读取 YAML，同时用 `CONTENTFLOW_` 环境变量覆盖 |

### 15. Weak Points or Limitations

- 启动装配文件较长，后续可考虑拆成更清晰的 wiring 函数，但本轮不改代码。
- scheduler interval、batch size、concurrency 目前主要是代码默认值，不在 config 中暴露。

## Flow: 登录和认证请求

### 1. Why This Flow Matters

大多数 API 都依赖登录后的 userID；理解认证后才能理解 source、article、collector 的用户隔离。

### 2. Entry Point

- File: `internal/module/auth/route.go`
- Function: `RegisterRoutes`
- Route: `POST /api/v1/auth/login`、`GET /api/v1/me`
- Trigger: 用户登录或访问受保护接口

### 3. Call Sequence

1. `auth.RegisterRoutes`
2. `auth.Handler.Login`
3. `auth.AuthService.Login`
4. `user.Repository.FindByEmail`
5. `verifyPassword`
6. `JWTTokenManager.GenerateAccessToken`
7. `JWTTokenManager.GenerateRefreshToken`
8. `RefreshTokenRepository.Create`
9. `setRefreshCookie`
10. 后续受保护请求进入 `middleware.AuthRequired`

### 4. Input and Output

| Step | Input | Output |
|---|---|---|
| Login handler | email、password JSON | service request |
| AuthService | normalized email、password | access token、refresh token、user |
| Refresh token repo | refresh token hash | `refresh_tokens` row |
| AuthRequired | `Authorization: Bearer <token>` | request context 中的 userID |

### 5. Data Structures

- `user.User`：账号数据。
- `auth.RefreshToken`：refresh token hash、过期时间、撤销时间。
- `auth.AccessTokenClaims`：JWT claim，包含 `uid` 和 email。

### 6. Error Handling

- 邮箱或密码错误都映射为 `invalid_credentials`。
- refresh token 无效映射为 `invalid_refresh_token`。
- middleware 缺 token 或 token 无效返回 401。

### 7. Persistence

使用 `users` 和 `refresh_tokens` 表。refresh token 原文返回给客户端或 cookie，数据库只保存 hash。

### 8. Cache / Lock / Rate Limit

登录 route 挂载 Redis 限流，key 是 `ratelimit:login:ip:<clientIP>`。

### 9. Async / Queue / Retry / Idempotency

认证流程没有异步队列。refresh token 使用旋转机制：刷新时撤销旧 token 并创建新 token。

### 10. Observability

HTTP metrics 记录 route、method、status、duration；GORM callbacks 记录数据库操作。

### 11. Important Edge Cases

- refresh token 可以从 JSON body 或 `contentflow_refresh_token` cookie 读取。
- cookie path 是 `/api/v1/auth`。
- `ErrRefreshTokenNotFound` 的错误字符串有拼写问题 `tokrn`，不影响流程，但面试中无需主动提。

### 12. 30-Second Interview Explanation

登录时 handler 校验请求，service 按 email 查用户并用 bcrypt 验证密码；成功后生成 JWT access token 和随机 refresh token，数据库只保存 refresh token hash。之后受保护接口通过 `AuthRequired` 解析 Bearer token，把 userID 放进 request context，业务模块用这个 userID 做数据隔离。

### 13. 2-Minute Interview Explanation

认证模块分为 handler、service、token manager 和 repository。handler 负责 HTTP 解析和错误码映射，service 负责邮箱规范化、密码校验、token 生成和 refresh token 生命周期，repository 负责用户和 refresh token 持久化。refresh 流程会先验证旧 refresh token hash，再撤销旧 token、生成新 access token 和新 refresh token。这个设计避免在数据库保存 refresh token 明文，同时让后续 source/article/collector 模块只依赖 request context 里的 userID。

### 14. Likely Interview Questions

| Question | Answer Outline |
|---|---|
| 为什么 refresh token 要存 hash？ | 数据库泄露时降低 token 原文被直接使用的风险 |
| access token 和 refresh token 区别？ | access token 短期认证请求，refresh token 用于换新 access token |
| userID 从哪来？ | JWT 解析后由 middleware 写入 request context |
| 登出做了什么？ | 按 refresh token hash 撤销 token 并清 cookie |

### 15. Weak Points or Limitations

- 没有角色/权限模型。
- 没有设备管理、全端登出 UI、refresh token 家族检测等更复杂机制。

## Flow: Source 采集到 Article 入库

### 1. Why This Flow Matters

这是项目最核心业务流：用户配置内容源，系统采集上游内容，去重后保存为文章。

### 2. Entry Point

- File: `internal/module/collector/route.go`
- Function: `RegisterRoutes`
- Route: `POST /api/v1/sources/:id/collect`
- Trigger: 用户手动采集；scheduler 也会调用同一个 service

### 3. Call Sequence

1. `collector.Handler.CollectSource`
2. `CollectionService.CollectSource`
3. `source.Repository.FindByUserIDAndID`
4. `Registry.Get(src.Type)`
5. `RedisCollectionLock.Acquire`
6. `RunRepository.Create`
7. `rss.Collector.Collect` 或 `email.Collector.Collect`
8. `article.Service.SaveCollectedItems`
9. `article.Repository.CreateIfNotExists`
10. `RunRepository.Finish`
11. `source.Repository.Update`
12. `CollectionObserver.ObserveCollection`

### 4. Input and Output

| Step | Input | Output |
|---|---|---|
| Handler | userID、sourceID | `CollectSourceRequest` |
| Source repo | userID、sourceID | scoped `Source` |
| Registry | source type | concrete collector |
| Collector | source config | `[]CollectedItem` |
| Article service | collected items | inserted / duplicated count |
| Run repo | final status | collection run row |

### 5. Data Structures

- `Source`：保存来源类型、URL、config_json、fetch 状态。
- `CollectedItem`：采集器统一输出。
- `Article`：入库文章。
- `CollectionRun`：采集执行记录。
- `ArticleWriteResult`：inserted / duplicated 计数。

### 6. Error Handling

- source 不存在时转换为 `ErrSourceNotAccessible`，HTTP 返回 404。
- collector 不存在返回 `collector_not_found`。
- 同 source 正在采集返回 409。
- 上游采集失败或写文章失败时，run 标记 failed，并返回带失败结果的 response。

### 7. Persistence

涉及表：

- `sources`：读取来源，并更新 last fetch 状态。
- `collection_runs`：记录 running、success、failed。
- `articles`：通过唯一索引避免重复。
- `article_states`：采集不直接写，用户读/收藏时写。

约束：

- `idx_articles_source_external_id`
- `idx_articles_source_content_hash`
- `idx_sources_user_url_active`

### 8. Cache / Lock / Rate Limit

- 采集 HTTP route 挂载 Redis 限流。
- `RedisCollectionLock` 使用 `SETNX` 加 source 级锁，释放时用 Lua 校验锁值。
- source list 和 article list cache 不在采集服务中直接清理；article 状态更新会清 article list cache。

### 9. Async / Queue / Retry / Idempotency

同步流程没有 Kafka。scheduler 调用的是同一个 `CollectSource`。幂等主要依赖文章唯一索引和 source 级锁。

### 10. Observability

- `CollectionService.observe` 写结构化日志。
- 如果 metrics 存在，调用 `Metrics.ObserveCollection` 记录 collection run 和 item counts。

### 11. Important Edge Cases

- RSS source 必须有 URL，且 URL 会经过 `netguard.ValidateHTTPURL` 防本地地址访问。
- Email source config 可以改变 `src.ConfigJSON` 中的 seen message IDs 和 last seen UID；之后 `sourceRepo.Update` 会保存这些变化。
- `CollectionService` 发现 `ErrCollectionFailed` 时，handler 会返回 200 和失败 run，而不是 500。

### 12. 30-Second Interview Explanation

采集入口是 `POST /sources/:id/collect`。handler 从认证 context 拿 userID，service 先用 userID 和 sourceID 查来源，再按 source type 找 RSS 或 Email collector，获取 Redis 锁，创建 running run，采集上游内容，转成统一的 `CollectedItem`，交给 article service 用唯一索引去重入库，最后更新 run 和 source 的抓取状态。

### 13. 2-Minute Interview Explanation

这个流程的关键是职责分层。collector service 不关心 RSS 和 Email 的解析细节，只依赖 `Collector` 接口；article service 不关心上游来源，只接收 `CollectedItem` 并转成 `Article`。去重不靠内存判断，而是数据库唯一索引和 `OnConflict DoNothing`。采集状态用 `collection_runs` 保存，source 上也有 last fetched status。为了避免同一个 source 并发采集，service 使用 Redis source lock；为了降低异常源风险，RSS HTTP fetch 会经过 netguard 校验，避免访问 localhost、private IP 等地址。

### 14. Likely Interview Questions

| Question | Answer Outline |
|---|---|
| 如何支持新 source type？ | 实现 `collector.Collector`，注册到 `Registry` |
| 去重在哪里做？ | `article.Repository.CreateIfNotExists` 和数据库唯一索引 |
| 采集失败如何记录？ | `finishFailed` 更新 `collection_runs` 和 source last fetch 状态 |
| 为什么需要 Redis 锁？ | 防止同一个 source 被并发采集导致重复写入或上游压力 |

### 15. Weak Points or Limitations

- 采集成功后没有显式删除 article list cache，列表缓存可能在 TTL 内看到旧数据。
- scheduler 参数没有配置化。
- RSS 采集默认限制 feed body 10MiB，但上游异常情况仍需更多运维保护。

## Flow: Kafka 异步采集和 DLQ

### 1. Why This Flow Matters

它展示项目如何把“用户请求”和“后台采集执行”拆开，并处理失败、重试和最终失败任务。

### 2. Entry Point

- File: `internal/app/server.go`
- Function: `Run()`
- Route: `POST /api/v1/sources/:id/collect`
- Trigger: `kafka.enabled=true` 时的采集请求

### 3. Call Sequence

1. `collector.AsyncHandler.RequestCollection`
2. `collectionjob.OutboxProducer.RequestCollection`
3. `OutboxRepository.Create`
4. `OutboxDispatcher.DispatchReady`
5. `KafkaWriter.Write`
6. `Worker.Run`
7. `Worker.HandleMessage`
8. `CollectionService.CollectSource`
9. success -> `TopicCollectionCompleted`
10. retryable failure -> `TopicCollectionFailed` + new `TopicCollectionRequested`
11. permanent/max failure -> `DLQRepository.Create` + `TopicCollectionDLQ`
12. DLQ API -> `DLQService.Replay` or `MarkHandled`

### 4. Input and Output

| Step | Input | Output |
|---|---|---|
| Async handler | userID、sourceID | HTTP 202 queued task |
| Outbox producer | collect request | `outbox_events` pending row |
| Dispatcher | ready outbox events | Kafka event and outbox status |
| Worker | `collection.requested` | completed/failed/retry/DLQ event |
| DLQ service | DLQ item id | replayed or handled state |

### 5. Data Structures

- `CollectionRequested`：异步采集请求。
- `OutboxEvent`：待投递事件。
- `Message`：Kafka reader 读出的消息。
- `CollectionCompleted` / `CollectionFailed`：结果事件。
- `DLQItem`：死信记录。

### 6. Error Handling

- dispatcher 写 Kafka 失败会 `MarkFailed` 并设置下次 attempt 时间。
- worker 采集失败先写 failed event。
- `IsPermanentError` 或达到 max attempts 后写 DLQ。
- DLQ replay 会重置 attempt 和 next attempt，再写回 `collection.requested`。

### 7. Persistence

涉及表：

- `outbox_events`：持久化待发送事件。
- `collection_dlq_items`：持久化死信任务。
- 采集本身仍写 `collection_runs` 和 `articles`。

### 8. Cache / Lock / Rate Limit

- API 入口仍有采集限流。
- worker 执行真实采集时仍使用 `CollectionService` 的 Redis source lock。

### 9. Async / Queue / Retry / Idempotency

- Kafka topic：`collection.requested`、`collection.completed`、`collection.failed`、`collection.dlq`。
- retry backoff：worker 默认 1 分钟指数退避，max attempts 默认 3，可由 config 覆盖。
- idempotency key：`collection:source:<sourceID>` 用作 Kafka key。
- 最终重复控制仍主要靠采集锁和 article 唯一索引。

### 10. Observability

- dispatcher 和 worker 调用 `ObserveJob` 记录 Kafka job metrics。
- collection service 仍记录 collection metrics。
- 日志记录 dispatcher、worker 和 collection 的失败。

### 11. Important Edge Cases

- outbox dispatcher 没有在代码里设置最大失败次数，会继续按 backoff 重试 ready failed events。
- worker commit 在 `HandleMessage` 后执行，即使 `HandleMessage` 返回错误也会 commit；失败语义依赖 worker 自己写 failed/retry/DLQ event。
- DLQ HTTP 接口只要求登录态，当前没有看到管理员权限或 user scope 过滤。

### 12. 30-Second Interview Explanation

Kafka 开启后，采集 API 不直接执行采集，而是把请求写入 outbox 并返回 queued。dispatcher 周期性扫描 ready outbox event 并写 Kafka。worker 消费 `collection.requested` 后调用同一个采集 service；成功写 completed，失败写 failed，再按 retry 规则重新发 requested，达到最大次数或永久错误后写 DLQ。

### 13. 2-Minute Interview Explanation

异步链路使用 outbox 把数据库写入和消息发送解耦，避免 API 进程在 Kafka 临时不可用时直接丢请求。outbox event 有 pending、sent、failed 状态，dispatcher 会对 failed event 做指数退避。worker 的核心方法是 `HandleMessage`：解析 `CollectionRequested`，必要时等到 `NextAttemptAt`，调用 `CollectionService.CollectSource`。失败时先写 failed event，再判断是否重试或进 DLQ。DLQ service 支持 list、replay、handled；replay 会把原 payload 重置后重新写到 requested topic。

### 14. Likely Interview Questions

| Question | Answer Outline |
|---|---|
| 为什么要 outbox？ | 避免请求和消息发送之间的失败窗口，先落库再投递 |
| retry 和 DLQ 怎么区分？ | retryable 失败继续 requested；永久错误或达到 max attempts 进 DLQ |
| 幂等如何做？ | Kafka key、source lock、article 唯一索引组合降低重复副作用 |
| 当前 DLQ 权限有什么风险？ | 只看到登录校验，没看到 admin 或 user scope 过滤 |

### 15. Weak Points or Limitations

- 不要声称 exactly-once。
- DLQ 管理权限需要加强。
- outbox 本身可以继续增加唯一键或幂等状态机。
