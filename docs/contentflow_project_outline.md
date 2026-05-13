# contentflow Go 后端项目路线图 Outline

> 项目目标：系统学习 Go 后端开发，并通过一个真实内容聚合系统串联 Gin、PostgreSQL、GORM、Redis、Kafka、JWT、测试、OpenAPI、Docker、可观测性、CI/CD、Kubernetes 等技术栈。
>
> 当前前端页面不在本路线内，前端可交给 AI 生成；后端提供稳定 API、OpenAPI 文档和完整测试。

---

## 状态说明

| 状态 | 含义 |
|---|---|
| 已推进 | 已在当前教学路线中完成设计与核心代码讲解 |
| 部分完成 | 已推进一部分，但仍有后续细节或测试未完成 |
| 待开始 | 还未进入 |
| 后续增强 | 不影响主线闭环，但接近真实生产项目时应补充 |

---

## 当前总览

| 阶段 | 模块 | 状态 |
|---|---|---|
| Stage 0 | 项目初始化与基础工程结构 | 已推进 |
| Stage 1 | Auth 认证模块 | 已推进 |
| Stage 2 | Source 内容源管理模块 | 已推进 |
| Stage 3 | Collector 抽象与采集编排 | 已推进 |
| Stage 4 | Article 文章入库与去重 | 已推进 |
| Stage 5 | RSS Collector | 已推进 |
| Stage 6 | 手动采集 API 与端到端链路 | 已推进 |
| Stage 7 | Email Collector | 已推进 |
| Stage 8 | Scheduler 定时采集 | 已推进 |
| Stage 9 | Kafka 异步采集架构 | 已推进 |
| Stage 10 | Redis 缓存与限流 | 待开始 |
| Stage 11 | Article 查询 API | 待开始 |
| Stage 12 | OpenAPI 文档 | 待开始 |
| Stage 13 | Observability：Prometheus / Grafana / OpenTelemetry | 待开始 |
| Stage 14 | Docker Compose 完整本地环境 | 部分完成 |
| Stage 15 | GitHub Actions CI | 待开始 |
| Stage 16 | Kubernetes 部署 | 待开始 |
| Stage 17 | AI 功能扩展 | 待开始 |
| Stage 18 | 项目收尾、压测、简历化总结 | 待开始 |

---

# Stage 0：项目初始化与基础工程结构

## 状态

已推进。

## 目标

搭建真实 Go 后端项目骨架，让项目具备可运行、可配置、可观测、可扩展的基础结构。

## 技术点

- Go module
- Gin
- Viper
- slog
- PostgreSQL
- Redis
- Docker Compose
- golang-migrate
- Makefile
- Graceful Shutdown

## 主要任务

- 初始化项目目录结构。
- 设计 `cmd/server/main.go` 入口。
- 设计 `internal/app/server.go` 作为应用装配层。
- 使用 Viper 加载 `config.yaml` 和环境变量。
- 初始化 slog 结构化日志。
- 初始化 PostgreSQL 连接。
- 初始化 Redis 连接。
- 初始化 Gin Router。
- 实现 `/healthz` 和 `/readyz`。
- 实现 HTTP Server graceful shutdown。
- 提供 Makefile 常用命令。
- 使用 Docker Compose 启动 PostgreSQL 和 Redis。

## 产出

- 可启动的后端服务。
- 基础工程目录。
- 配置系统。
- 日志系统。
- 数据库连接。
- Redis 连接。
- 健康检查接口。
- 本地运行脚本。

## 核心理解点

- 为什么不要把所有初始化逻辑写进 `main.go`。
- 为什么 Viper 不建议直接使用全局实例。
- 为什么 `gin.New()` 通常比 `gin.Default()` 更适合真实项目。
- 为什么生产项目需要 graceful shutdown。
- `/healthz` 和 `/readyz` 的区别。

---

# Stage 1：Auth 认证模块

## 状态

已推进。

## 目标

实现用户认证闭环，为后续用户级资源隔离提供基础。

## 技术点

- Gin Handler
- JWT
- Refresh Token
- bcrypt
- GORM
- PostgreSQL
- gomock
- httptest
- table-driven tests

## 主要任务

- 设计 User model。
- 设计 RefreshToken model。
- 使用 migration 创建 `users`、`refresh_tokens` 表。
- 实现 UserRepository。
- 实现 RefreshTokenRepository。
- 实现 AuthService：
  - Register
  - Login
  - Refresh
  - Logout
  - Me
- 实现 JWT TokenManager。
- Refresh Token 原文只返回客户端，数据库只保存 hash。
- 实现 Auth Handler。
- 实现 Auth Routes。
- 实现 Auth Middleware。
- 使用标准库 `context.Context` 存储 user_id。
- 使用自定义 context key 类型，避免 key 冲突。
- 为 AuthService 编写 table-driven tests。
- 为 AuthHandler 编写 table-driven tests。
- 为 AuthMiddleware 编写 table-driven tests。
- 为 TokenManager 编写 table-driven tests。
- 编写阶段 Markdown 学习笔记。

## API

```http
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/me
```

## 产出

- 用户注册。
- 用户登录。
- Access Token。
- Refresh Token Rotation。
- Logout 撤销 refresh token。
- `/me` 获取当前用户。
- Auth Middleware。
- Auth 模块测试。

## 核心理解点

- 为什么 Refresh Token 不能明文入库。
- 为什么登录失败不区分“用户不存在”和“密码错误”。
- 为什么密码不能用 SHA256，而应该用 bcrypt / argon2 / scrypt。
- 为什么 Handler / Service / Repository 要分层。
- 为什么 Service DTO 不写 `json` / `binding` tag。
- 为什么 Middleware 不应该 import 业务包，避免循环依赖。
- 为什么 context 里存值使用自定义 key 类型。

## 后续增强

- Refresh Token rotation 使用事务。
- 登录失败限流。
- 多设备登录管理。
- Access Token 黑名单或 token version。
- 更强密码策略。
- Repository 集成测试。

---

# Stage 2：Source 内容源管理模块

## 状态

已推进。

## 目标

让用户可以管理自己的内容来源，支持 RSS 和 Email source，并为后续采集系统做准备。

## 技术点

- GORM
- PostgreSQL
- Soft Delete
- Partial Unique Index
- Gin
- gomock
- testcontainers
- table-driven tests

## Source 类型

- `rss`
- `email`

## 主要任务

- 设计 Source model。
- 使用 migration 创建 `sources` 表。
- 支持 `type + config_json` 扩展不同内容源。
- 实现 SourceRepository：
  - Create
  - FindByUserIDAndID
  - ListByUserID
  - Update
  - SoftDelete
- 实现 SourceService：
  - CreateSource
  - ListSources
  - GetSource
  - UpdateSource
  - DeleteSource
- 实现 SourceHandler。
- 实现 SourceRoutes。
- Source 查询必须带 `user_id`。
- 删除 Source 使用软删除，不物理删除。
- 使用 partial unique index 限制同一用户重复添加同一未删除 URL。
- 为 SourceHandler 编写 table-driven tests。
- 为 SourceService 编写 table-driven tests。
- 为 SourceRepository 编写 testcontainers integration tests。
- 编写阶段 Markdown 学习笔记。

## API

```http
POST   /api/v1/sources
GET    /api/v1/sources
GET    /api/v1/sources/:id
PATCH  /api/v1/sources/:id
DELETE /api/v1/sources/:id
```

## 产出

- 用户级 Source CRUD。
- 多租户隔离。
- 软删除。
- `rss` / `email` source 类型。
- `config_json` 扩展配置。
- Source 模块测试。

## 核心理解点

- 为什么所有 Source 查询必须带 `user_id`。
- 为什么删除 Source 用软删除。
- 为什么 `ON DELETE CASCADE` 只在物理删除时触发。
- 为什么 soft delete 是 UPDATE，不会删除 articles。
- 为什么 `is_active` 和 `deleted_at` 都需要。
- 为什么 partial unique index 支持软删除后重新添加相同 URL。
- 为什么 Repository 集成测试要用真实 PostgreSQL。

## 后续增强

- Source config_json schema 校验。
- Source 恢复接口。
- RSS URL 可访问性检查。
- Email source 强类型配置。

---

# Stage 3：Collector 抽象与采集编排

## 状态

已推进。

## 目标

建立统一采集抽象，让 RSS、Email、GitHub、Hacker News 等来源都能接入同一条采集链路。

## 技术点

- Interface 设计
- Registry 模式
- 任务状态机
- gomock
- table-driven tests

## 主要任务

- 设计 Collector 接口。
- 设计 CollectedItem。
- 实现 Collector Registry。
- 为 Registry 编写 table-driven tests。
- 设计 CollectionRun model。
- 实现 RunRepository。
- 设计 ArticleWriter 接口。
- 实现 CollectionService 编排：
  - 查询 Source
  - 根据 Source.Type 找 Collector
  - 创建 collection_run
  - 调用 Collector.Collect
  - 调用 ArticleWriter.SaveCollectedItems
  - 更新 collection_run 状态
  - 更新 source last fetch 状态
- 为 CollectionService 编写 table-driven tests。

## 核心接口

```go
type Collector interface {
    Type() string
    Collect(ctx context.Context, src *source.Source) ([]CollectedItem, error)
}
```

```go
type ArticleWriter interface {
    SaveCollectedItems(ctx context.Context, items []CollectedItem) (*ArticleWriteResult, error)
}
```

## 产出

- Collector 插件化架构。
- Registry 注册表。
- CollectionService 编排层。
- CollectionRun 状态记录。
- Collector 和 ArticleWriter 解耦。

## 核心理解点

- 为什么 Collector 不直接写 articles 表。
- 为什么 CollectionService 不直接写 RSS / Email 抓取逻辑。
- 为什么 Registry 可以避免 `switch source.Type`。
- 为什么采集流程不适合包在一个长事务里。
- 为什么任务状态适合用 collection_runs 记录。
- 为什么 HTTP 手动采集、Scheduler 定时采集、Kafka Worker 异步采集都应该复用 CollectionService。

## 后续增强

- running 超时任务修复。
- collection_run 更完整的错误分类。
- 采集任务幂等控制。
- 并发采集控制。

---

# Stage 4：Article 文章入库与去重

## 状态

已推进。

## 目标

把采集到的 CollectedItem 保存为 Article，并通过数据库唯一索引实现幂等去重。

## 技术点

- GORM
- PostgreSQL
- `ON CONFLICT DO NOTHING`
- Unique Index
- testcontainers
- table-driven tests

## 主要任务

- 设计 Article model。
- 使用 migration 创建 `articles` 表。
- 实现 ArticleRepository：
  - CreateIfNotExists
- 实现 ArticleService：
  - SaveCollectedItems
- ArticleService 实现 Collector 模块定义的 ArticleWriter 接口。
- 根据 `external_id` 去重。
- 根据 `content_hash` 去重。
- 统计 inserted_count。
- 统计 duplicated_count。
- 为 ArticleService 编写 table-driven tests。
- 为 ArticleRepository 编写 testcontainers integration tests。

## 核心方法

```go
CreateIfNotExists(ctx, article) (created bool, err error)
```

```go
SaveCollectedItems(ctx, items) (*collector.ArticleWriteResult, error)
```

## 去重约束

```sql
UNIQUE(source_id, external_id) WHERE external_id IS NOT NULL
UNIQUE(source_id, content_hash)
```

## 产出

- 文章入库能力。
- 幂等插入。
- 重复文章跳过。
- inserted / duplicated 统计。
- CollectionService 可以真正写入 articles 表。

## 核心理解点

- 为什么不能“先查再插入”。
- 为什么要用 `ON CONFLICT DO NOTHING`。
- 为什么 external_id 和 content_hash 都需要。
- 为什么当前 content_hash 去重范围是 source 内部，而不是全局。
- 为什么 Repository 去重逻辑必须用真实 PostgreSQL 集成测试。

## 后续增强

- 批量插入优化。
- 全局内容去重。
- URL canonicalization。
- SimHash / MinHash / embedding 相似度去重。
- Article 查询 API。

---

# Stage 5：RSS Collector

## 状态

已推进。

## 已完成内容

- RSSCollector 设计。
- 使用 `mmcdole/gofeed` 作为 Feed parser。
- 实现 RSSCollector 基础转换逻辑。
- ExternalID 规则：
  - 优先 GUID
  - 其次 Link
- ContentHash 生成。
- 处理 title / link / author / summary / content / published_at。
- Parser 抽象。
- 进一步重构 Fetcher 抽象：
  - Fetcher 负责 HTTP 获取 feed body。
  - Parser 负责解析 io.Reader。
  - Collector 负责转换 CollectedItem。
- HTTP 请求使用 `http.NewRequestWithContext`。
- 默认 HTTP timeout。
- User-Agent。
- `io.LimitReader` 限制最大 feed size。
- 更新 RSSCollector tests，适配 Fetcher + Parser 新结构。
- 测试 Fetcher 失败。
- 测试 Parser 失败。
- 测试 body close。
- 测试 item 字段转换。
- 测试 content_hash 稳定性。
- 测试 context cancellation。
- 测试非 2xx HTTP 状态。
- 将 RSSCollector 注册到 app.Run 的 Collector Registry。
- 使用本地 HTTP feed + PostgreSQL integration test 验证 RSS Source 端到端采集链路。

## 技术点

- gofeed
- HTTP client
- context cancellation
- interface 抽象
- io.Reader / io.ReadCloser
- table-driven tests

## 产出目标

- 可真实抓取 RSS / Atom / JSON Feed。
- 转换为统一 CollectedItem。
- 接入 CollectionService。
- 通过 ArticleService 入库。

## 核心理解点

- 为什么 `ParseURL` 不适合直接放在生产级 Collector。
- 为什么 Fetcher 和 Parser 要拆开。
- 为什么 HTTP 请求要使用 `http.NewRequestWithContext`。
- 为什么 Fetcher 返回 `io.ReadCloser`，而不是 `[]byte`。
- 为什么 Collector 仍然不应该写数据库。

---

# Stage 6：手动采集 API 与端到端链路

## 状态

已推进。

## 目标

提供手动触发采集接口，验证 Source → Collector → ArticleService → articles 表完整链路。

## 主要任务

- 实现 CollectionHandler。
- 实现 CollectionRoutes。
- API 通过 Auth Middleware 保护。
- 从 request context 获取 user_id。
- 调用 CollectionService.CollectSource。
- 返回 collection run 结果。
- 编写 Handler table-driven tests。
- 编写端到端手动验证脚本。

## API

```http
POST /api/v1/sources/:id/collect
```

## 产出

- 用户可以手动采集一个 Source。
- RSS Source 可以被抓取并写入 articles。
- collection_runs 记录成功 / 失败状态。
- source.last_fetched_at 被更新。

## 核心理解点

- 为什么手动采集、定时采集、异步采集都应复用 CollectionService。
- 为什么 Handler 不直接调用 RSSCollector。
- 为什么接口返回 run 统计，而不是直接返回所有文章。

---

# Stage 7：Email Collector

## 状态

已推进。

## 目标

支持通过邮箱订阅接收内容，并将邮件内容转换为 CollectedItem。

## 方案选择

可选方案：

1. IMAP 拉取共享邮箱。
2. Gmail API。
3. 邮件 webhook / inbound email provider。
4. 用户专属 alias 地址。

建议学习阶段优先：

```text
IMAP / shared mailbox
```

因为它更容易本地模拟和理解。

## 主要任务

- 设计 Email Source config。
- 定义 EmailCollector。
- 定义 MailboxReader 抽象，用于后续接入 IMAP/shared mailbox adapter。
- 根据 from_filter / recipient_alias 过滤邮件。
- Message-ID 作为 ExternalID。
- Subject 作为 Title。
- Body 作为 Content。
- From 作为 Author。
- Date 作为 PublishedAt。
- 生成 ContentHash。
- 编写 EmailCollector tests。
- 将 EmailCollector 注册到 app.Run 的 Collector Registry。

## 产出

- email source 可以接入统一 Collector。
- newsletter 邮件可以转换为文章。
- 支持用户通过邮箱订阅内容源。

## 核心理解点

- Email 是被动接收，RSS 是主动拉取。
- Email source 可以没有 URL。
- Email 的 ExternalID 通常来自 Message-ID。
- 邮件正文清理比 RSS 更复杂。

---

# Stage 8：Scheduler 定时采集

## 状态

已推进。

## 目标

让系统自动定时采集 active sources。

## 技术点

- goroutine
- ticker
- context cancellation
- slog
- graceful shutdown
- worker pool

## 主要任务

- 设计 SourceRepository.ListActive。
- 实现 Scheduler。
- 定时扫描 active sources。
- 调用 CollectionService。
- 限制并发采集数量。
- 记录日志。
- 处理 graceful shutdown。
- 编写 Scheduler tests。

## 产出

- 系统启动后自动定时采集。
- active source 会周期性更新文章。
- Scheduler 支持关闭信号。

## 核心理解点

- 为什么 Scheduler 只负责任务调度，不负责采集细节。
- 为什么要限制并发。
- 为什么 Scheduler 查询要同时判断 `is_active = true` 和 `deleted_at IS NULL`。
- 为什么定时任务要支持 context cancellation。

---

# Stage 9：Kafka 异步采集架构

## 状态

已推进。

## 目标

把同步采集改造成异步任务，提高接口响应速度和系统解耦能力。

## 技术点

- Kafka Producer
- Kafka Consumer
- Consumer Group
- Topic
- Offset
- DLQ
- Idempotency
- Outbox Pattern

## 主要任务

- 设计 Kafka topic：
  - `collection.requested`
  - `collection.completed`
  - `collection.failed`
  - `collection.dlq`
- 手动 collect API 在启用 Kafka 后不直接采集，而是发送采集任务。
- Worker 消费采集任务。
- Worker 调用 CollectionService。
- 设计任务 idempotency key。
- 明确 Kafka 可能重复消费，下游仍依赖文章去重约束保证幂等。
- 失败重试。
- DLQ。
- 编写 Kafka 本地验证脚本。

## 产出

- HTTP 接口快速返回任务已提交。
- Worker 异步执行采集。
- 支持重试和失败队列。
- 为后续分布式 worker 做准备。

## 核心理解点

- 为什么 Kafka 不能消除重复消费。
- 为什么下游必须做幂等。
- 为什么 Outbox 用于解决 DB 与 Kafka 的一致性问题。
- 为什么 DLQ 是失败隔离机制。
- 为什么 Consumer Group 的 offset 是按 group 维护的。

---

# Stage 10：Redis 缓存与限流

## 状态

待开始。

## 目标

使用 Redis 提升读取性能，并保护认证与采集接口。

## 技术点

- Redis
- cache-aside
- TTL
- distributed rate limiting
- Lua script
- sliding window / token bucket

## 主要任务

- 缓存最近文章列表。
- 缓存 source 列表。
- 登录接口限流。
- 采集接口限流。
- Redis key 设计。
- 缓存失效策略。
- Lua 实现原子限流。
- 编写 Redis 集成测试。

## 产出

- 高频列表接口读取更快。
- 登录暴力破解风险降低。
- 手动采集接口被保护。
- Redis 使用场景清晰。

## 核心理解点

- Redis 不应该作为主持久化存储。
- 缓存和数据库不一致是常态，需要策略管理。
- 限流必须原子执行。
- 缓存 key 设计要包含 user_id，避免用户数据串扰。

---

# Stage 11：Article 查询 API

## 状态

待开始。

## 目标

让前端可以查询文章列表、搜索文章、标记阅读状态和收藏状态。

## 技术点

- Gin
- GORM
- PostgreSQL indexes
- Pagination
- Search
- User article state

## 主要任务

- 设计 ArticleRepository 查询方法。
- 设计 ArticleService 查询方法。
- 实现文章列表 API。
- 支持 source_id 过滤。
- 支持 q 搜索。
- 支持 is_read / is_saved 过滤。
- 实现 article_states。
- 标记已读。
- 收藏 / 取消收藏。
- 编写 Handler / Service / Repository 测试。

## API

```http
GET    /api/v1/articles
GET    /api/v1/articles/:id
PATCH  /api/v1/articles/:id/read
PATCH  /api/v1/articles/:id/save
```

## 产出

- 前端可以展示文章流。
- 用户可以管理阅读状态和收藏状态。
- 文章数据真正可用。

## 核心理解点

- 为什么 article_states 是 user 与 article 的关系表。
- 为什么 article_states 懒创建。
- 为什么查询文章列表时要 LEFT JOIN article_states。
- 为什么没有 state 记录时默认未读 / 未收藏。

---

# Stage 12：OpenAPI 文档

## 状态

待开始。

## 目标

为后端 API 生成清晰、可交付给前端或 AI 前端生成器使用的接口文档。

## 技术点

- OpenAPI 3.0
- swagger
- API schema
- error response schema

## 主要任务

- 定义 OpenAPI 文件。
- 覆盖 Auth API。
- 覆盖 Source API。
- 覆盖 Collection API。
- 覆盖 Article API。
- 定义统一响应结构。
- 定义统一错误结构。
- 接入 Swagger UI 或 Redoc。
- CI 中校验 OpenAPI 文件。

## 产出

- `openapi.yaml`
- 可交给前端 AI 使用的 API 契约。
- API 文档页面。

## 核心理解点

- OpenAPI 是前后端契约。
- DTO 变更需要同步文档。
- 统一错误响应能显著降低前端处理复杂度。

---

# Stage 13：Observability：Prometheus / Grafana / OpenTelemetry

## 状态

待开始。

## 目标

让系统具备真实项目中的可观测性能力。

## 技术点

- Prometheus
- Grafana
- OpenTelemetry
- Metrics
- Tracing
- Structured Logging

## 主要任务

- 增加 request_id。
- 增加 HTTP request metrics。
- 增加 collector metrics。
- 增加 DB latency metrics。
- 增加 Kafka metrics。
- 接入 OpenTelemetry trace。
- 配置 Prometheus scrape。
- 配置 Grafana dashboard。
- 采集 CollectionService 链路 trace。

## 产出

- `/metrics`
- Grafana dashboard。
- trace 链路。
- 结构化日志带 request_id / user_id / source_id / run_id。

## 核心理解点

- Metrics 用于看趋势和报警。
- Logs 用于定位具体问题。
- Traces 用于跨服务链路分析。
- 可观测性不是最后才补，而应该逐步嵌入关键路径。

---

# Stage 14：Docker Compose 完整本地环境

## 状态

部分完成。

## 已完成

- PostgreSQL。
- Redis。

## 待完成

- Kafka。
- Prometheus。
- Grafana。
- OpenTelemetry Collector。
- Backend service。
- Migration runner。
- 本地一键启动脚本。

## 目标

让整个后端系统可以一键本地启动。

## 主要任务

- 完善 docker-compose.yaml。
- 添加 backend service。
- 添加 Kafka / Redpanda。
- 添加 Prometheus。
- 添加 Grafana。
- 添加 otel-collector。
- 添加 Makefile：
  - compose-up
  - compose-down
  - migrate-up
  - migrate-down
  - logs
  - test
  - test-integration

## 产出

- 一键本地开发环境。
- 新机器可快速启动项目。

---

# Stage 15：GitHub Actions CI

## 状态

待开始。

## 目标

让项目具备基础工程质量保障。

## 技术点

- GitHub Actions
- go test
- go vet
- golangci-lint
- Docker build
- Integration tests

## 主要任务

- 配置 Go test。
- 配置 lint。
- 配置 build。
- 配置 migration 检查。
- 可选：跑 testcontainers 集成测试。
- 可选：构建 Docker image。

## 产出

- PR 自动测试。
- 自动 lint。
- 自动构建。
- 更接近真实团队项目。

---

# Stage 16：Kubernetes 部署

## 状态

待开始。

## 目标

把 contentflow 部署到 Kubernetes，学习生产部署基础。

## 技术点

- Kubernetes Deployment
- Service
- Ingress
- ConfigMap
- Secret
- Job
- CronJob
- HPA
- Readiness / Liveness Probe

## 主要任务

- 编写 backend Deployment。
- 编写 backend Service。
- 编写 Ingress。
- 编写 ConfigMap。
- 编写 Secret。
- 编写 migration Job。
- 编写 Scheduler / Worker Deployment。
- 配置 health probe。
- 配置资源 request / limit。
- 本地使用 kind 验证。
- 可选：Helm Chart。

## 产出

- contentflow 可部署到 Kubernetes。
- 支持 backend / worker 分离。
- 支持配置和 Secret 管理。
- 支持健康检查和滚动更新。

## 核心理解点

- Deployment 管理无状态服务。
- Job 更适合 migration。
- Secret 不应该写进镜像。
- readiness 决定是否接流量。
- liveness 决定是否重启容器。

---

# Stage 17：AI 功能扩展

## 状态

待开始。

## 背景

用户提到后续可能参考 `Thysrael/Horizon` 引入 AI 功能。

## 目标

在内容聚合系统上增加 AI 能力，而不是单独做一个孤立 AI demo。

## 可选功能

- 文章摘要。
- 每日 Digest。
- 主题聚类。
- 文章标签生成。
- 阅读优先级评分。
- RAG 问答。
- 用户兴趣画像。
- 推荐系统。
- 相似文章检测。

## 技术点

- Python AI service 或 Go 调用 LLM API。
- Embedding。
- Vector DB。
- RAG。
- Async jobs。
- Kafka。
- Redis。
- PostgreSQL pgvector。

## 建议阶段

```text
17.1 AI summary job
17.2 Article embedding
17.3 Similar article grouping
17.4 Daily digest
17.5 RAG search
17.6 Recommendation scoring
```

## 核心理解点

- AI 功能应基于已有 Article 数据。
- AI 任务适合异步处理。
- LLM 输出需要缓存。
- Embedding 和原文需要版本管理。
- AI 结果最好作为可重算的派生数据。

---

# Stage 18：项目收尾、压测、简历化总结

## 状态

待开始。

## 目标

把项目整理成可展示、可部署、可写进简历的后端项目。

## 主要任务

- 完善 README。
- 完善架构图。
- 完善 OpenAPI 文档。
- 完善部署文档。
- 补齐核心测试。
- 使用 wrk / k6 压测核心接口。
- 写性能优化记录。
- 写故障排查记录。
- 整理项目亮点。
- 形成简历 bullet points。

## 简历亮点方向

- Go + Gin 构建内容聚合后端系统。
- PostgreSQL + GORM + migration 管理核心数据。
- JWT + Refresh Token 实现认证闭环。
- Redis 实现缓存与限流。
- Kafka 实现异步采集任务。
- Collector 插件化架构支持 RSS / Email 等多内容源。
- testcontainers 实现 Repository 集成测试。
- Prometheus + OpenTelemetry 实现可观测性。
- Docker Compose / Kubernetes 支持部署。

---

# 建议后续推进顺序

当前最合理的下一步：

```text
1. 完成 Stage 5.4：更新 RSSCollector Tests
2. 完成 Stage 6：手动采集 API
3. 完成 Stage 11 的 Article 查询 API 基础版
4. 再进入 Stage 8 Scheduler
5. 再进入 Stage 9 Kafka 异步化
```

原因：

```text
先打通同步端到端链路，再做异步化。
先确保功能正确，再用 Kafka 解耦性能。
```

也就是：

```text
同步正确性
  ↓
手动采集可用
  ↓
文章可查询
  ↓
定时采集
  ↓
Kafka 异步化
  ↓
缓存 / 观测 / 部署
```
