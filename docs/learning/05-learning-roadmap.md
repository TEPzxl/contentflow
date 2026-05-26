# 学习路线图

## 0. 术语预解释

- 阶段：一组可以在 30 到 90 分钟内完成的学习任务。
- 目标：本阶段结束时你应该能解释或操作的内容。
- 阅读顺序：为了降低理解成本而安排的文件阅读路径。
- 自检问题：用来判断自己是否真的理解，而不是只看过代码。
- 面试模板：把源码理解转成可口头表达的结构。

## Stage 1: 入口、配置和运行模式

### Goal

知道程序从哪里启动、如何读取配置、如何决定启动 API、scheduler、worker。

### Why This Stage Comes First

如果不先理解 `app.Run()`，后面的模块都只是散点，无法知道它们如何被组装到一起。

### Required Files

- `cmd/server/main.go`
- `internal/app/server.go`
- `internal/app/mode.go`
- `internal/config/config.go`
- `configs/config.yaml`
- `configs/config.docker.yaml`

### Required Functions / Structs / Interfaces

- `main`
- `app.Run`
- `runtimePlanForMode`
- `config.Config`
- `config.Load`

### Reading Order

1. `cmd/server/main.go`
2. `internal/app/mode.go`
3. `internal/config/config.go`
4. `internal/app/server.go`

### Commands to Run

- `go test ./internal/app ./internal/config`
- `go run ./cmd/server`，仅在本地 PostgreSQL 和 Redis 可用时运行。

### Diagram to Draw

画出：`main -> app.Run -> config -> db/redis -> services -> router/server/background workers`。

### Notes to Write

- `app.mode` 的四种模式。
- Kafka 开关如何改变采集 route。
- 哪些依赖在启动时必须可用。

### Self-Check Questions

- `worker` 模式为什么要求 `kafka.enabled=true`？
- `CONTENTFLOW_CONFIG` 不设置时读取哪个文件？
- Kubernetes backend、worker、scheduler 如何用同一个镜像跑不同模式？

### Interview Explanation Template

“项目入口很薄，只负责调用 `app.Run()`。真正的装配在 `internal/app/server.go`，它读取配置、初始化数据库和 Redis、创建各模块 service 和 handler，再根据运行模式启动 HTTP、scheduler 和 worker。”

### Do Not Go Deep Into

- 先不要读每个业务 service 的全部实现。
- 先不要启动 Docker Compose。

## Stage 2: HTTP 路由、认证和请求上下文

### Goal

理解请求如何从 Gin router 进入 handler，以及登录后 userID 如何传递给业务模块。

### Required Files

- `internal/http/router.go`
- `internal/http/middleware/auth.go`
- `internal/http/requestctx/context.go`
- `internal/module/auth/route.go`
- `internal/module/auth/handler.go`
- `internal/module/auth/service.go`
- `internal/module/auth/token.go`

### Required Functions / Structs / Interfaces

- `NewRouter`
- `AuthRequired`
- `AuthService.Login`
- `JWTTokenManager`
- `RefreshTokenRepository`

### Reading Order

1. `internal/http/router.go`
2. `internal/module/auth/route.go`
3. `internal/module/auth/handler.go`
4. `internal/module/auth/service.go`
5. `internal/http/middleware/auth.go`

### Commands to Run

- `go test ./internal/http/... ./internal/module/auth/...`

### Diagram to Draw

画出：`POST /auth/login -> handler -> service -> user repo -> token manager -> refresh token repo`。

### Notes to Write

- access token 和 refresh token 的区别。
- userID 放进 request context 的位置。
- 认证错误如何映射成 HTTP status。

### Self-Check Questions

- 为什么 refresh token 只保存 hash？
- `GET /me` 为什么不用请求 body？
- Bearer token 格式不正确时在哪里返回 401？

### Interview Explanation Template

“认证模块把 HTTP 解析、业务校验和 token 管理拆开。登录成功后返回 access token 并设置 refresh cookie；受保护 route 通过 middleware 解析 Bearer token，把 userID 写入 request context，后续模块用它做用户隔离。”

### Do Not Go Deep Into

- 暂时不扩展 OAuth、RBAC、组织权限。

## Stage 3: Source 管理和数据库边界

### Goal

理解用户如何创建 RSS / Email source，以及 source 如何校验、脱敏、缓存和软删除。

### Required Files

- `internal/module/source/route.go`
- `internal/module/source/handler.go`
- `internal/module/source/service.go`
- `internal/module/source/repository.go`
- `internal/module/source/model.go`
- `internal/module/source/cache.go`
- `migrations/000001_create_initial_tables.up.sql`

### Required Functions / Structs / Interfaces

- `SourceService.CreateSource`
- `normalizeURL`
- `redactConfig`
- `Repository.FindByUserIDAndID`
- `RedisListCache`
- `Source`

### Reading Order

1. migration 中的 `sources` 表
2. `model.go`
3. `route.go`
4. `handler.go`
5. `service.go`
6. `repository.go`
7. `cache.go`

### Commands to Run

- `go test ./internal/module/source/...`

### Diagram to Draw

画出：`POST /sources -> auth middleware -> handler -> service validation -> repository -> cache invalidation`。

### Notes to Write

- source 的 user scope 如何保证。
- URL 安全校验为什么重要。
- config 脱敏覆盖哪些 key。

### Self-Check Questions

- RSS source 为什么必须有 URL？
- source 删除为什么是 soft delete？
- source list cache 的 key 由哪些字段组成？

### Interview Explanation Template

“source 模块负责内容来源的生命周期。创建时会规范化 name/type/url/config，RSS URL 还要通过 netguard 防止访问本地或私有地址。查询和更新都按 userID 过滤，返回配置前会做敏感字段脱敏。”

### Do Not Go Deep Into

- 先不要研究 RSS/Email 采集细节。

## Stage 4: 同步采集、RSS/Email 和文章入库

### Goal

理解从 source 到 article 的核心业务流。

### Required Files

- `internal/module/collector/service.go`
- `internal/module/collector/collector.go`
- `internal/module/collector/registry.go`
- `internal/module/collector/run_repository.go`
- `internal/module/collector/lock.go`
- `internal/module/collector/rss/collector.go`
- `internal/module/collector/email/collector.go`
- `internal/module/article/service.go`
- `internal/module/article/repository.go`

### Required Functions / Structs / Interfaces

- `CollectionService.CollectSource`
- `Collector`
- `Registry`
- `RedisCollectionLock`
- `rss.Collector.Collect`
- `email.Collector.Collect`
- `article.Service.SaveCollectedItems`
- `Repository.CreateIfNotExists`

### Reading Order

1. `collector.Collector` interface
2. `collector.Registry`
3. `CollectionService.CollectSource`
4. RSS collector
5. Email collector
6. `article.Service.SaveCollectedItems`
7. `article.Repository.CreateIfNotExists`

### Commands to Run

- `go test ./internal/module/collector/... ./internal/module/article/...`

### Diagram to Draw

画出：`source -> collector -> collected items -> article service -> articles table -> collection_runs`。

### Notes to Write

- 采集成功和失败分别如何更新 run。
- 文章去重依赖哪些唯一索引。
- Redis lock 的 key 和释放逻辑。

### Self-Check Questions

- collector service 为什么不直接依赖 RSS collector？
- 采集失败为什么也能返回 collection run？
- `CreateIfNotExists` 如何判断 inserted 和 duplicated？

### Interview Explanation Template

“采集服务按 source type 从 registry 找具体 collector，拿到 source 锁后创建 running run。collector 输出统一的 collected item，article service 再转换成 Article 并通过数据库唯一约束去重。最后 run 和 source fetch 状态都会更新。”

### Do Not Go Deep Into

- 暂时不看 Kafka 和 DLQ。

## Stage 5: Article 查询、全文搜索和状态更新

### Goal

理解文章如何按用户隔离查询、如何搜索、如何标记已读和收藏。

### Required Files

- `internal/module/article/route.go`
- `internal/module/article/handler.go`
- `internal/module/article/service.go`
- `internal/module/article/repository.go`
- `internal/module/article/model.go`
- `internal/module/article/cache.go`
- `migrations/000004_create_article_search_index.up.sql`

### Required Functions / Structs / Interfaces

- `Handler.List`
- `Service.ListArticles`
- `Repository.ListByUser`
- `applyArticleFilters`
- `Repository.UpsertState`
- `ArticleWithState`

### Reading Order

1. migration 中的 `articles` 和 `article_states`
2. `model.go`
3. `handler.go`
4. `service.go`
5. `repository.go`
6. search index migration

### Commands to Run

- `go test ./internal/module/article`

### Diagram to Draw

画出：`GET /articles -> query filters -> JOIN sources/article_states -> DTO`。

### Notes to Write

- 列表为什么不返回 content。
- 全文搜索使用哪个 PostgreSQL 表达式。
- read/save 状态为什么用 upsert。

### Self-Check Questions

- 查询文章如何保证只能看到自己的 source 下的 article？
- `is_read` 和 `is_saved` 过滤在哪里实现？
- 更新状态后为什么要删除 article list cache？

### Interview Explanation Template

“文章查询不是直接按 article 表查，而是 JOIN sources 确认 source 属于当前 user，再 LEFT JOIN article_states 取已读和收藏状态。列表会省略正文，详情才返回 content；状态更新使用 upsert 维护 user 和 article 的唯一状态行。”

### Do Not Go Deep Into

- 先不优化搜索排序和分页策略。

## Stage 6: Kafka outbox、worker、retry 和 DLQ

### Goal

理解异步采集如何把 API 请求转为后台任务，以及失败如何重试或进入 DLQ。

### Required Files

- `internal/module/collector/async_handler.go`
- `internal/module/collectionjob/outbox.go`
- `internal/module/collectionjob/outbox_repository.go`
- `internal/module/collectionjob/event.go`
- `internal/module/collectionjob/worker.go`
- `internal/module/collectionjob/dlq.go`
- `internal/module/collectionjob/dlq_repository.go`
- `internal/module/collectionjob/dlq_handler.go`
- `migrations/000002_create_kafka_reliability_tables.up.sql`

### Required Functions / Structs / Interfaces

- `OutboxProducer.RequestCollection`
- `OutboxDispatcher.DispatchReady`
- `Worker.HandleMessage`
- `IsPermanentError`
- `DLQService.Replay`
- `DLQService.MarkHandled`

### Reading Order

1. event definitions
2. async handler
3. outbox producer
4. outbox dispatcher
5. worker
6. DLQ service and handler
7. migration

### Commands to Run

- `go test ./internal/module/collectionjob`

### Diagram to Draw

画出：`API -> outbox_events -> dispatcher -> Kafka -> worker -> collector -> completed/failed/retry/DLQ`。

### Notes to Write

- 哪些失败进入 retry，哪些失败进 DLQ。
- outbox 解决了什么失败窗口。
- DLQ replay 会如何重置 event。

### Self-Check Questions

- worker 为什么在 `HandleMessage` 后 commit？
- outbox dispatcher 写 Kafka 失败后如何处理？
- `idempotency_key` 当前能保证什么，不能保证什么？

### Interview Explanation Template

“异步模式下，API 不直接采集，而是写 outbox 并返回 queued。dispatcher 扫描 ready event 发送 Kafka，worker 消费后调用同一个采集 service。失败时先写 failed，再按 max attempts 和 permanent error 判断重试或写 DLQ。”

### Do Not Go Deep Into

- 不要声称 exactly-once。

## Stage 7: AI 模块和可替换 Assistant

### Goal

理解 AI 功能的真实边界：默认是本地算法，不是外部 LLM。

### Required Files

- `internal/module/ai/handler.go`
- `internal/module/ai/service.go`
- `internal/module/ai/repository.go`
- `internal/module/ai/assistant.go`
- `internal/module/ai/worker.go`
- `internal/module/ai/model.go`
- `migrations/000003_create_ai_feature_tables.up.sql`
- `migrations/000005_add_embedding_lookup_index.up.sql`

### Required Functions / Structs / Interfaces

- `Assistant`
- `ExtractiveAssistant`
- `Service.RequestSummary`
- `Service.ProcessNextSummary`
- `Service.GenerateEmbedding`
- `Service.SimilarArticles`
- `Service.GenerateDigest`
- `Service.RAGSearch`

### Reading Order

1. AI routes in `handler.go`
2. `Assistant` interface
3. `ExtractiveAssistant`
4. summary request and worker flow
5. embedding and similarity
6. digest and RAG search
7. repository and migrations

### Commands to Run

- `go test ./internal/module/ai`

### Diagram to Draw

画出：`POST /articles/:id/summary -> enqueue -> SummaryWorker -> Assistant -> article_summaries`。

### Notes to Write

- 哪些能力是同步生成，哪些能力是队列处理。
- embedding 如何存储。
- RAGSearch 如何找到文章和生成引用。

### Self-Check Questions

- 默认 Assistant 为什么不需要 API key？
- SimilarArticles 为什么会先生成 target embedding？
- RAGSearch 为什么不等于完整向量数据库 RAG？

### Interview Explanation Template

“AI 模块通过 Assistant 接口抽象 provider，默认实现是本地 extractive/hash 算法。摘要先入队，后台 worker claim pending job 后生成摘要；embedding 用 hash token vector 存到 PostgreSQL JSONB；相似文章用 cosine similarity；RAG 搜索先用文章全文搜索，再返回本地生成答案和 citations。”

### Do Not Go Deep Into

- 不要把默认实现说成外部大模型。

## Stage 8: 前端、部署、监控和验证

### Goal

理解用户如何通过前端使用系统，以及项目如何被本地和容器环境启动、监控、验证。

### Required Files

- `web/app/page.tsx`
- `web/features/workbench/workbench.tsx`
- `web/lib/api/client.ts`
- `web/lib/auth/session.ts`
- `deployments/docker-compose.yaml`
- `deployments/k8s/base/backend-deployment.yaml`
- `deployments/k8s/base/worker-deployment.yaml`
- `deployments/k8s/base/scheduler-deployment.yaml`
- `internal/observability/metrics.go`
- `internal/observability/tracing.go`
- `.github/workflows/ci.yaml`

### Required Functions / Structs / Interfaces

- `Workbench`
- `api`
- `readSession`
- `Metrics.HTTPMiddleware`
- `Metrics.ObserveCollection`
- `InitTracing`

### Reading Order

1. `web/app/page.tsx`
2. `workbench.tsx`
3. `api/client.ts`
4. `session.ts`
5. Docker Compose
6. K8s backend/worker/scheduler deployments
7. metrics/tracing
8. CI workflow

### Commands to Run

- `npm --prefix web run typecheck`
- `npm --prefix web run lint`
- `go test ./internal/observability ./internal/http`
- `docker compose -f deployments/docker-compose.yaml up --build`，仅在你想启动完整本地栈时运行。

### Diagram to Draw

画出：`Browser -> Next.js Workbench -> API client -> Gin API -> Postgres/Redis/Kafka`。

### Notes to Write

- access token 存在哪里。
- refresh token 如何由浏览器带给后端。
- backend、worker、scheduler 在 K8s 里如何分模式。

### Self-Check Questions

- Compose 里前端为什么映射到 3001？
- `/metrics` 何时注册？
- CI workflow 覆盖哪些类别？

### Interview Explanation Template

“前端是一个工作台，不是营销页。它用 sessionStorage 保存 access token，API client 自动加 Bearer token，并在 401 时尝试 refresh。部署层有 Compose 本地环境和 K8s backend/worker/scheduler 拆分，观测层通过 Prometheus metrics、OpenTelemetry tracing 和 Grafana dashboard 配置支撑排查。”

### Do Not Go Deep Into

- 本轮不深入 UI 样式。
- 本轮不实际部署 K8s。
