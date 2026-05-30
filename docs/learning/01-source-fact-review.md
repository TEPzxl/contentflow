# 源码事实审查

## 0. 术语预解释

- 源码事实：只根据仓库里的代码、配置、脚本、测试和文档能证明的结论。
- 入口点：程序开始执行，或者用户开始访问某个功能时进入的文件、函数或命令。
- 模块：一组围绕同一职责组织的代码目录。
- 能力：项目对外提供或内部实际执行的一项功能。
- 证据：能支持结论的具体文件、函数、配置或测试。
- 置信度：当前结论有多少源码支撑；高表示有多处源码或测试支持，中表示源码存在但还未运行验证，低表示只看到局部线索。

## 1. 一句话项目描述

contentflow 是一个以 Go 后端和 Next.js 前端组成的内容聚合系统，用于管理 RSS / Email 来源、采集文章、查询文章、维护阅读状态，并提供异步采集、失败处理、监控和本地算法版 AI 能力。

证据：

- `cmd/server/main.go` 调用 `internal/app.Run()` 启动后端。
- `internal/app/server.go` 组装 auth、source、collector、article、collectionjob、ai、scheduler、observability 等模块。
- `web/app/page.tsx` 渲染 `Workbench`，`web/features/workbench/workbench.tsx` 组合认证、来源、文章、采集记录和 AI 面板。
- `migrations/*.sql` 创建 users、sources、articles、collection_runs、outbox_events、collection_dlq_items、article_summaries、article_embeddings、daily_digests 等表。

## 2. 实际项目类型

分类：

- Web 应用：有后端 HTTP API 和前端工作台。
- 内容聚合系统：核心模型是 source、article、collection run。
- 本地可运行的工程化示例：有 Docker Compose、Kubernetes 配置、监控、测试和 CI。
- 不应直接称为已上线生产系统：仓库中有生产形态配置，但没有真实线上运行、真实流量、真实 SLO 达成记录或外部部署证据。

## 3. 入口点

| Entry | File | Function / Command | Evidence |
|---|---|---|---|
| 后端程序入口 | `cmd/server/main.go` | `main()` -> `app.Run()` | 真实可执行入口 |
| 应用组装入口 | `internal/app/server.go` | `Run()` | 加载配置、连接数据库、Redis、注册模块、启动 HTTP / scheduler / worker |
| 路由入口 | `internal/http/router.go` | `NewRouter()` | 注册 `/api/v1`、`/healthz`、`/readyz`、`/metrics`、API docs |
| 运行模式入口 | `internal/app/mode.go` | `runtimePlanForMode()` | 支持 `all`、`api`、`scheduler`、`worker` |
| 前端入口 | `web/app/page.tsx` | `HomePage()` | 渲染 `Workbench` |
| 本地容器入口 | `deployments/docker-compose.yaml` | `docker compose ... up` | 启动 backend、frontend、postgres、redis、kafka、migrate、otel、jaeger、prometheus、grafana |
| 数据库结构入口 | `migrations/*.sql` | migration up/down | 定义主要表和索引 |
| Kubernetes 入口 | `deployments/k8s/base/*.yaml` | Deployment / Service / Job / HPA | 定义 backend、frontend、worker、scheduler、migration job |

## 4. 已实现能力

| Capability | Evidence | Confidence | Notes |
|---|---|---|---|
| 用户注册、登录、刷新、登出、当前用户查询 | `internal/module/auth/*`、`internal/module/user/repository.go`、`migrations/000001...` | 高 | 密码用 bcrypt，access token 用 JWT，refresh token 只存 hash |
| Source 管理 | `internal/module/source/handler.go`、`service.go`、`repository.go` | 高 | 支持 RSS / Email 类型、软删除、用户隔离、敏感配置脱敏 |
| RSS 采集 | `internal/module/collector/rss/collector.go` | 高 | HTTP 拉取、gofeed 解析、内容 hash 去重输入 |
| Email 采集 | `internal/module/collector/email/collector.go`、`imap_reader.go`、`directory_reader.go` | 中 | 支持 empty / directory / imap provider，真实邮箱依赖配置 |
| 手动同步采集 | `collector.RegisterRoutes()`、`CollectionService.CollectSource()` | 高 | 默认 `kafka.enabled=false` 时走同步采集 |
| Kafka 异步采集 | `internal/module/collectionjob/*`、`configs/config.docker.yaml` | 高 | Docker / K8s 配置启用 Kafka，代码有 outbox、worker、retry、DLQ |
| Collection run 记录 | `collection_runs` 表、`collector/run_repository.go` | 高 | 成功/失败都记录 fetched、inserted、duplicated、error |
| Article 查询和状态 | `internal/module/article/*` | 高 | 支持列表、详情、全文搜索、已读、收藏 |
| Source 和 Article 列表缓存 | `internal/module/source/cache.go`、`internal/module/article/cache.go` | 高 | Redis list cache，写操作后按用户删除缓存 |
| 登录和采集限流 | `internal/ratelimit/limiter.go`、`internal/app/server.go` | 高 | Redis Lua 脚本，登录按 IP，采集按 user + source |
| AI 摘要、embedding、相似文章、digest、RAG 搜索 | `internal/module/ai/*` | 高 | 默认是本地 extractive/hash 算法，不是外部 LLM |
| Prometheus metrics | `internal/observability/metrics.go`、`internal/http/router.go` | 高 | HTTP、collection、Kafka job、GORM metrics |
| OpenTelemetry tracing | `internal/observability/tracing.go`、`internal/http/router.go` | 中 | 配置打开时接入 otelgin 和 OTLP exporter |
| 前端工作台 | `web/features/workbench/workbench.tsx`、`web/lib/api/client.ts` | 高 | 有登录、来源、文章、采集记录、AI 视图 |
| Docker Compose 本地栈 | `deployments/docker-compose.yaml` | 高 | 配置明确，是否在本机成功启动本轮未验证 |
| Kubernetes 部署文件 | `deployments/k8s/base`、`overlays` | 高 | 有 base/dev/staging/prod，是否实际部署成功本轮未验证 |
| CI / release workflow | `.github/workflows/ci.yaml`、`.github/workflows/release-images.yaml`、`scripts/ci.sh` | 高 | workflow 调用 `scripts/ci.sh`，本地也可按分项运行测试、校验和构建 |

## 5. 部分实现或需要谨慎表述的能力

| Capability | Evidence | Missing Pieces | Safe Wording |
|---|---|---|---|
| 生产级部署 | K8s manifests、HPA、ExternalSecret、release workflow | 没有真实集群运行证据、线上指标、灾备演练 | “具备容器化和 Kubernetes 部署清单” |
| AI 能力 | `ai.NewExtractiveAssistant()` | 默认不是外部大模型，不依赖真实 LLM provider | “实现了可替换 Assistant 接口和本地算法版摘要/向量/RAG” |
| RAG 搜索 | `Service.RAGSearch()` 使用文章查询和本地 `Answer()` | 不是向量数据库检索，也不是大模型回答 | “基于文章全文搜索和本地引用拼接的 RAG 形态接口” |
| Email 真实采集 | `ConfiguredMailboxReader` 支持 `imap` | 需要真实邮箱配置和凭据；本轮未运行 | “支持 IMAP 配置路径和目录模拟路径” |
| 异步采集幂等 | Kafka key、article unique index、Redis lock | outbox event 本身没有唯一键；重放语义需谨慎 | “通过文章唯一约束和采集锁降低重复写入风险” |
| 可观测性 | metrics、tracing、Grafana dashboard | 本轮未启动 Prometheus / Grafana / Jaeger 验证 | “代码和部署配置接入监控与追踪” |

## 6. README / 文档 / 源码差异

| Source Claim | Code Reality | Risk | Recommendation |
|---|---|---|---|
| “AI APIs” | 默认实现是 `ExtractiveAssistant`，本地摘要和 hash embedding | 容易被误解为接入外部 LLM | 面试中主动说明默认 provider 是本地可测试实现 |
| “RAG 搜索” | `RAGSearch` 先调用文章全文搜索，再本地生成答案和 citations | 容易被误解为向量检索 + LLM | 表述为 RAG 风格接口，而不是完整 LLM RAG 系统 |
| “Kubernetes dev/staging/prod overlays” | manifests 存在，脚本可渲染检查 | 没有实际集群运行证据 | 只说有部署清单和渲染校验 |
| “CI 覆盖 web-test 等” | workflow 调用 `scripts/ci.sh`，脚本覆盖 Go、OpenAPI、migration、K8s、Web 类型检查/构建/测试等检查项 | 本轮未完整运行所有 CI 分支 | 不基于未运行的 CI 分支下更强结论 |

## 7. 不应夸大的能力

- 不要说“已支撑真实生产流量”，仓库没有流量证据。
- 不要说“接入真实 OpenAI / 外部 LLM”，默认实现不是。
- 不要说“完整分布式 exactly-once 采集”，代码中主要是 Redis lock、Kafka key、retry、DLQ、数据库唯一约束的组合。
- 不要说“企业级权限系统”，目前认证是用户登录和 Bearer token；没有角色、组织、管理员权限模型。
- 不要说“DLQ 管理有管理员隔离”，`DLQHandler` 要求登录并按 user scope 过滤 list/replay/handled，但没有角色、组织或管理员权限模型。

## 8. 最安全的面试描述

这是一个内容聚合项目。我从入口启动链路开始，把 Go 后端拆成认证、来源管理、采集、文章、异步任务、AI 和可观测性模块；用户可以登录后维护 RSS 或 Email 来源，系统可以同步或通过 Kafka 异步触发采集，把采集结果按文章唯一约束入库，并记录 collection run。项目还包含 Redis 缓存和限流、Kafka outbox/retry/DLQ、Prometheus 指标、OpenTelemetry tracing、Next.js 工作台、Docker Compose 和 Kubernetes 部署清单。AI 部分默认不是外部大模型，而是用本地 extractive/hash Assistant 实现摘要、embedding、相似文章、digest 和 RAG 风格搜索，便于无密钥测试和后续替换 provider。

## 9. 不安全的面试说法

- “我做了生产级内容平台”：应降级为“具备生产形态组件和部署清单的完整学习/项目工程”。
- “我接入了 LLM 做 RAG”：应降级为“实现了 Assistant 接口和 RAG 风格流程，默认 provider 是本地算法”。
- “Kafka 保证消息 exactly-once”：应降级为“用 outbox、retry、DLQ、文章唯一约束和锁降低失败与重复风险”。
- “有完整权限体系”：应降级为“有用户认证和资源按用户隔离；管理类权限仍需加强”。
