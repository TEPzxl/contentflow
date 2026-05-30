# 源码地图

## 0. 术语预解释

- 目录：文件系统中按职责组织代码的位置。
- 职责：某个目录或文件负责解决的问题。
- 依赖方向：一个模块调用另一个模块时形成的方向。
- 数据结构：代码中用来承载数据的 struct、DTO 或 database model。
- 接口：Go 中定义行为契约的 `interface`，调用方只依赖方法集合，不依赖具体实现。
- DTO：Data Transfer Object，用于接口层和服务层之间传递数据的结构。

## 1. 目录概览

| Path | Responsibility |
|---|---|
| `cmd/server` | 后端二进制入口 |
| `internal/app` | 应用生命周期、依赖装配、运行模式 |
| `internal/config` | 配置读取、默认值、安全校验 |
| `internal/database` | PostgreSQL 连接 |
| `internal/cache` | Redis 连接 |
| `internal/http` | Gin router、middleware、统一响应、健康检查 |
| `internal/module/auth` | 注册、登录、JWT、refresh token |
| `internal/module/user` | 用户表 repository |
| `internal/module/source` | 内容来源 CRUD、校验、缓存 |
| `internal/module/collector` | 采集编排、run 记录、锁、同步和异步 HTTP handler |
| `internal/module/collector/rss` | RSS feed 拉取和解析 |
| `internal/module/collector/email` | Email 来源读取和转换 |
| `internal/module/article` | 文章入库、查询、阅读/收藏状态、缓存 |
| `internal/module/collectionjob` | Kafka event、outbox、worker、retry、DLQ |
| `internal/module/scheduler` | 定时扫描 active sources 并触发采集 |
| `internal/module/ai` | 摘要、embedding、相似文章、digest、RAG 风格搜索 |
| `internal/ratelimit` | Redis 限流中间件 |
| `internal/netguard` | 出站 URL / 地址安全校验 |
| `internal/observability` | metrics、tracing、GORM callbacks |
| `migrations` | 数据库表和索引变更 |
| `api` | OpenAPI 文档和测试 |
| `web` | Next.js 前端 |
| `deployments` | Docker Compose、Dockerfile、K8s、Prometheus、Grafana、OTel 配置 |
| `.github/workflows` | CI 和镜像发布 workflow |

## 2. 模块职责表

| Module | Path | Responsibility | Core Files | Core Structs / Functions | Dependencies |
|---|---|---|---|---|---|
| Runtime | `internal/app` | 组装依赖并启动 HTTP/scheduler/worker | `server.go`, `mode.go` | `Run`, `runtimePlanForMode` | config, database, cache, all modules |
| HTTP | `internal/http` | 创建 router、中间件、健康检查、响应格式 | `router.go`, `middleware/*`, `handler/health.go` | `NewRouter`, `AuthRequired`, `HealthHandler` | Gin, Redis, GORM |
| Auth | `internal/module/auth` | 用户认证和 token 生命周期 | `service.go`, `handler.go`, `token.go`, `refresh_token_repository.go` | `AuthService`, `JWTTokenManager`, `RefreshTokenRepository` | user repo, bcrypt, JWT, DB |
| User | `internal/module/user` | 用户表读写 | `repository.go`, `model.go` | `User`, `Repository` | GORM |
| Source | `internal/module/source` | 来源 CRUD、校验、脱敏、列表缓存 | `service.go`, `repository.go`, `cache.go`, `type.go` | `SourceService`, `Source`, `RedisListCache` | GORM, Redis, netguard |
| Collector | `internal/module/collector` | 同步采集编排、run 记录、锁、HTTP handler | `service.go`, `run_repository.go`, `lock.go`, `route.go` | `CollectionService`, `Collector`, `CollectionRun`, `RedisCollectionLock` | source repo, article writer, Redis |
| RSS Collector | `internal/module/collector/rss` | 拉取和解析 RSS / Atom | `collector.go` | `Collector`, `HTTPFetcher`, `GofeedParser` | gofeed, netguard |
| Email Collector | `internal/module/collector/email` | 读取邮箱或目录消息并转换为采集项 | `collector.go`, `imap_reader.go`, `directory_reader.go` | `ConfiguredMailboxReader`, `IMAPMailboxReader` | source config |
| Article | `internal/module/article` | 文章去重入库、列表、详情、状态更新、搜索 | `service.go`, `repository.go`, `cache.go`, `model.go` | `Service`, `Repository`, `Article`, `ArticleState` | GORM, Redis |
| Collection Job | `internal/module/collectionjob` | Kafka 事件、outbox、worker、DLQ | `outbox.go`, `worker.go`, `dlq.go`, `producer.go` | `OutboxProducer`, `OutboxDispatcher`, `Worker`, `DLQService` | Kafka writer/reader, DB |
| Scheduler | `internal/module/scheduler` | 定时批量采集 active source | `scheduler.go` | `Scheduler.RunOnce`, `SourceLister`, `CollectionService` | source repo, collector service |
| AI | `internal/module/ai` | 摘要、embedding、digest、RAG 风格搜索 | `service.go`, `repository.go`, `assistant.go`, `worker.go` | `Service`, `Assistant`, `ExtractiveAssistant`, `SummaryWorker` | article repo, DB |
| Observability | `internal/observability` | 指标和 tracing | `metrics.go`, `tracing.go` | `Metrics`, `InitTracing` | Prometheus, OpenTelemetry, GORM |
| Frontend | `web` | 浏览器工作台 | `app/page.tsx`, `features/*`, `lib/api/client.ts` | `Workbench`, `api`, `readSession` | Next.js, React |

## 3. 依赖方向

推荐按这个方向理解：

1. `cmd/server/main.go` 只调用 `internal/app.Run()`。
2. `internal/app.Run()` 读取配置，创建数据库、Redis、token manager、service、handler、scheduler、worker。
3. `internal/http.NewRouter()` 接收 route registration function，把业务 handler 挂到 `/api/v1`。
4. HTTP handler 从 request context 取 `userID`，解析请求，再调用 service。
5. Service 执行业务规则，调用 repository、cache、collector、assistant 或 event producer。
6. Repository 用 GORM 读写 PostgreSQL。
7. Cache / lock / rate limit 用 Redis。
8. Kafka 模式下，API 先写 outbox，dispatcher 发 Kafka，worker 再调用 collector service。
9. 前端通过 `web/lib/api/client.ts` 调用后端 API。

不要反向理解为“数据库调用 service”或“repository 调用 handler”。这个项目的主方向是：入口 -> app -> router -> handler -> service -> repository / external system。

## 4. 核心数据结构

| Name | File | Purpose | Used By |
|---|---|---|---|
| `User` | `internal/module/user/model.go` | 用户账号 | auth, user repository |
| `RefreshToken` | `internal/module/auth/model.go` | refresh token hash 和过期/撤销状态 | auth service |
| `Source` | `internal/module/source/model.go` | RSS / Email 来源配置 | source, collector, scheduler |
| `Article` | `internal/module/article/model.go` | 采集后的文章内容 | article, collector, ai |
| `ArticleState` | `internal/module/article/model.go` | 用户对文章的已读/收藏状态 | article service |
| `CollectionRun` | `internal/module/collector/model.go` | 一次采集执行记录 | collector, run API |
| `CollectedItem` | `internal/module/collector/item.go` | collector 输出、article service 输入 | RSS / Email collector, article service |
| `CollectionRequested` | `internal/module/collectionjob/event.go` | 异步采集请求事件 | outbox, worker, DLQ |
| `OutboxEvent` | `internal/module/collectionjob/outbox.go` | 待发送事件 | outbox dispatcher |
| `DLQItem` | `internal/module/collectionjob/dlq.go` | 多次失败后的采集任务 | DLQ service |
| `ArticleSummary` | `internal/module/ai/model.go` | 摘要任务和结果 | ai service, summary worker |
| `ArticleEmbedding` | `internal/module/ai/model.go` | 文章向量 | ai service |
| `DailyDigest` | `internal/module/ai/model.go` | 每日摘要 | ai service |

## 5. 核心接口

| Interface | File | Purpose | Implementations |
|---|---|---|---|
| `auth.Service` | `internal/module/auth/service.go` | 认证业务契约 | `AuthService` |
| `auth.TokenManager` | `internal/module/auth/token.go` | access / refresh token 生成和解析 | `JWTTokenManager` |
| `source.Repository` | `internal/module/source/repository.go` | source 持久化 | `GormRepository` |
| `source.ListCache` | `internal/module/source/service.go` | source 列表缓存 | `RedisListCache` |
| `collector.Collector` | `internal/module/collector/collector.go` | 一种 source type 的采集器 | RSS collector, Email collector |
| `collector.ArticleWriter` | `internal/module/collector/service.go` | 保存采集项 | `article.Service` |
| `collector.CollectionLock` | `internal/module/collector/service.go` | source 级采集锁 | `RedisCollectionLock` |
| `article.Repository` | `internal/module/article/repository.go` | 文章读写和状态 upsert | `GormRepository` |
| `collectionjob.EventWriter` | `internal/module/collectionjob/producer.go` | 写 Kafka 风格事件 | `KafkaWriter`, tests fakes |
| `collectionjob.EventReader` | `internal/module/collectionjob/worker.go` | 读 Kafka 风格消息 | `KafkaReader` |
| `collectionjob.OutboxRepository` | `internal/module/collectionjob/outbox.go` | outbox 表访问 | `GormOutboxRepository` |
| `collectionjob.DLQRepository` | `internal/module/collectionjob/dlq.go` | DLQ 表访问 | `GormDLQRepository` |
| `ai.Assistant` | `internal/module/ai/assistant.go` | AI provider 抽象 | `ExtractiveAssistant` |
| `ai.Repository` | `internal/module/ai/repository.go` | AI 表访问 | `GormRepository` |
| `ratelimit.Limiter` | `internal/ratelimit/limiter.go` | 限流判断 | `RedisLimiter` |

## 6. 推荐阅读顺序

1. `cmd/server/main.go`
2. `internal/app/mode.go`
3. `internal/app/server.go`
4. `internal/http/router.go`
5. `migrations/000001_create_initial_tables.up.sql`
6. `internal/module/auth/route.go`、`handler.go`、`service.go`
7. `internal/module/source/route.go`、`handler.go`、`service.go`、`repository.go`
8. `internal/module/collector/service.go`
9. `internal/module/collector/rss/collector.go`
10. `internal/module/article/service.go`、`repository.go`
11. `internal/module/collectionjob/outbox.go`、`worker.go`、`dlq.go`
12. `internal/module/ai/service.go`、`assistant.go`
13. `web/lib/api/client.ts`、`web/features/workbench/workbench.tsx`
14. `deployments/docker-compose.yaml`、`deployments/k8s/base/*.yaml`

## 7. 先读文件

- `cmd/server/main.go`：确认真实启动入口。
- `internal/app/server.go`：确认系统如何装配。
- `internal/http/router.go`：确认 API 如何进入业务模块。
- `migrations/000001_create_initial_tables.up.sql`：理解核心实体关系。
- `internal/module/collector/service.go`：理解最核心业务流。

## 8. 暂缓阅读文件

- `internal/module/*/mocks/*`：先不用读，等看测试时再读。
- `*_benchmark_test.go`：性能基准测试，先理解功能后再看。
- `deployments/grafana/dashboards/*.json`：仪表盘 JSON 很长，先理解 metrics 名称即可。
- `api/openapi.yaml`：适合对照路由，不适合第一遍直接通读。
- `web/app/globals.css`、UI 样式文件：先理解 API 和状态流，再看样式。
