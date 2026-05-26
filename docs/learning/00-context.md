# Learning Context

## How to Read This File

本文档是学习该项目之前的上下文确认文件。它只记录已经从源码或仓库文件中核验过的信息，以及继续学习前必须由你确认的信息。

你的要求：

- 不默认你已经懂任何技术名词。
- 每个技术名词首次出现前，先用一句话解释它是什么。
- 不对你的目标、背景、时间安排或优先级做假设。
- 有不确定性时先说明并提问。

## Terminology Used in This Context

- Go：一种常用于服务端程序的编程语言，适合编写并发网络服务和命令行程序。
- 后端：运行在服务器侧、负责处理请求、业务逻辑、数据读写和外部系统交互的程序部分。
- 前端：运行在浏览器侧、负责页面展示和用户交互的程序部分。
- API：应用程序接口，通常指前端或其他系统通过 HTTP 请求调用后端能力的约定。
- HTTP：浏览器和服务端常用的网络通信协议。
- Gin：一个 Go 语言的 HTTP Web 框架，用来定义路由、中间件和请求处理逻辑。
- GORM：一个 Go 语言的数据库访问库，用结构体映射数据库表并执行查询。
- PostgreSQL：一种关系型数据库，用表、行、列保存结构化数据。
- Redis：一种内存型数据存储，常用于缓存、限流、锁等高频读写场景。
- Kafka：一种消息队列系统，用来把任务或事件异步发送给后台消费者处理。
- 迁移：数据库结构变更脚本，用来创建或修改表、索引和约束。
- Next.js：一个基于 React 的前端框架，用来构建网页应用。
- React：一个前端 UI 库，用组件方式组织页面。
- TypeScript：在 JavaScript 基础上加入类型检查的语言。
- Docker Compose：一个本地编排工具，用配置文件一次启动多个容器服务。
- Kubernetes：一个容器编排系统，用来在集群中部署、扩缩容和管理服务。
- OpenTelemetry：一套可观测性标准，用来采集链路追踪等运行时信息。
- Prometheus：一种指标采集和查询系统，常用于监控服务状态。
- Grafana：一种监控看板工具，用图表展示 Prometheus 等系统的数据。
- DLQ：Dead Letter Queue，死信队列，用来保存多次处理失败后暂时不能继续处理的消息。
- RAG：Retrieval-Augmented Generation，检索增强生成，通常指先检索相关内容再交给 AI 生成答案。
- Entry point：入口点，指程序开始执行或用户开始访问某个能力的文件、函数或命令。
- Business code：业务代码，指实现项目功能和行为的代码，而不是学习文档、说明文档或纯配置说明。

## Repository

- Name: contentflow
- Path: `/home/tep/dev/real_project/contentflow`
- Primary language:
  - 已核验：Go 后端，证据是 `go.mod` 和 `cmd/server/main.go`。
  - 已核验：TypeScript / React / Next.js 前端，证据是 `web/package.json`。
- Frameworks / libraries observed:
  - 后端 HTTP 框架：Gin，证据是 `go.mod` 和 `internal/http/router.go`。
  - 后端数据库访问：GORM，证据是 `go.mod` 和 `internal/app/server.go`。
  - 前端框架：Next.js、React，证据是 `web/package.json`。
  - 缓存或限流相关：Redis，证据是 `go.mod` 和 `internal/app/server.go`。
  - 异步任务相关：Kafka，证据是 `go.mod` 和 `internal/app/server.go`。
  - 可观测性相关：Prometheus、OpenTelemetry、Grafana，证据是 `go.mod`、`internal/http/router.go`、`deployments/grafana/`。
- Entry points:
  - 后端命令入口：`cmd/server/main.go`，调用 `internal/app.Run()`。
  - 后端路由入口：`internal/http/router.go`，创建 Gin router，并注册 `/api/v1`、`/healthz`、`/readyz`、`/metrics` 和 API docs。
  - 前端应用入口：`web/app/page.tsx` 和 `web/app/layout.tsx`。
  - 本地 Docker Compose 入口：`deployments/docker-compose.yaml`。
  - 常用命令入口：`Makefile` 和 `scripts/ci.sh`。

## User Goal

- Source learning: 已确认。你明确要求我使用 existing project learning coach 带领你学习该项目。
- Interview preparation: 已确认。学习资料需要包含面试准备。
- Resume verification: 不需要。你已说明简历已完成，本轮不做简历 bullet 验证。
- Second-stage development: 不需要。本轮目标不是二次开发规划。
- Technical remediation: 不需要。本轮不改业务代码，也不修复技术债。

## Constraints

- Business code modification allowed: 不允许。你已明确禁止修改业务代码。
- Documentation-only mode: 是。仅允许创建或更新 `docs/learning/` 下的 Markdown 文件。
- Learning files target directory: `docs/learning/`。
- Learning horizon: 不绑定日期的分阶段路线。
- Prioritized modules: 从零开始，按入口到核心业务流的顺序学习。
- Resume bullets provided: 不需要。你已说明简历已完成。
- Current worktree note: 发现已有未提交改动 `internal/module/collector/rss_integration_test.go` 和 `scripts/ci.sh`。本次没有读取或修改这两个文件的内容。

## Initial Source Facts Verified

- 该仓库不是只有 README 的空项目；它包含 Go 后端、Next.js 前端、数据库迁移、Docker Compose、Kubernetes 配置、OpenAPI 文档、监控配置和测试文件。
- 后端实际启动路径是 `cmd/server/main.go -> internal/app.Run()`。
- `internal/app/server.go` 会加载配置、初始化日志、追踪、PostgreSQL、Redis、认证、限流、source/article/collector/AI 等模块，并按运行模式启动 HTTP 服务、调度器、worker 或 outbox dispatcher。
- `internal/http/router.go` 负责创建 HTTP 路由，并注册健康检查、就绪检查、指标接口和 API 文档。
- README 声明了 RSS / Email 内容源、采集、去重、入库、查询、收藏、异步重试、DLQ、AI 摘要、相似文章、Daily Digest、RAG 搜索等功能；这些声明后续必须逐项对照源码验证，不能只按 README 采信。

## Missing Information

当前没有阻塞继续生成学习资料的信息。

仍需在后续学习中保持的不确定性：

- 不假设你已掌握任何技术名词；首次出现时先解释。
- 不假设 README 中的功能都已完整实现；后续每项能力都需要源码证据。
- 不运行可能修改数据、启动外部服务或依赖密钥的命令，除非你明确允许。

## Generated Learning Package

已生成或更新：

- `docs/learning/01-source-fact-review.md`
- `docs/learning/02-project-boundary.md`
- `docs/learning/03-source-map.md`
- `docs/learning/04-core-flows.md`
- `docs/learning/05-learning-roadmap.md`
- `docs/learning/06-resume-bullet-verification.md`，本轮记录为跳过实质验证。
- `docs/learning/07-interview-qa.md`
- `docs/learning/08-runbook.md`
- `docs/learning/stages/01-entry-runtime.md`
- `docs/learning/stages/02-http-auth-context.md`
- `docs/learning/stages/03-source-boundary.md`
- `docs/learning/stages/04-collection-article.md`
- `docs/learning/stages/05-article-query-state.md`
- `docs/learning/stages/06-kafka-outbox-dlq.md`
- `docs/learning/stages/07-ai-assistant.md`
- `docs/learning/stages/08-frontend-deploy-observability.md`
