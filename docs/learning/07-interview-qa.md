# 面试准备

## 0. 术语预解释

- 面试表述：把源码事实用清晰、不过度夸大的方式讲给面试官。
- 架构：系统如何拆分模块、模块之间如何协作。
- 高频问题：面试中常被追问的设计、边界和取舍问题。
- 风险问题：如果回答过满，容易暴露源码理解不足的问题。

## 1. 项目 30 秒版本

contentflow 是一个内容聚合系统。我负责理解和梳理它的 Go 后端、Next.js 前端和部署配置。后端从 `cmd/server/main.go` 进入 `app.Run()`，按配置启动 API、scheduler 和 worker；核心能力包括用户认证、RSS/Email source 管理、采集、文章去重入库、文章查询、Redis 缓存和限流、Kafka outbox/retry/DLQ、Prometheus/OpenTelemetry 观测，以及本地算法版 AI 摘要、embedding、相似文章和 RAG 风格搜索。

## 2. 项目 2 分钟版本

这个项目的主线是“用户维护内容来源，系统采集并组织文章”。启动层由 `internal/app` 统一装配配置、数据库、Redis、Kafka、HTTP route、scheduler 和 worker。HTTP 层用 Gin，中间件负责 request id、认证、安全响应头、日志、metrics 和 tracing。认证模块用 bcrypt 校验密码，用 JWT 做 access token，用 hash 后的 refresh token 做刷新和登出。

业务上，source 模块负责 RSS/Email 来源 CRUD、URL 安全校验、配置脱敏和列表缓存。采集模块通过 `Collector` 接口支持 RSS 和 Email，采集时会拿 Redis source lock、写 collection run、调用具体 collector，再把统一的 `CollectedItem` 交给 article service 入库。article 模块用数据库唯一索引做去重，支持列表、详情、全文搜索、已读和收藏状态。

如果 Kafka 开启，采集请求不直接执行，而是写 outbox，dispatcher 投递 `collection.requested`，worker 消费后调用同一个采集 service；失败会写 failed event，按 backoff retry，达到最大次数或永久错误后进入 DLQ。AI 模块通过 Assistant 接口隔离 provider，默认实现是本地 extractive/hash 算法，不是外部 LLM。部署层提供 Docker Compose、Kubernetes manifests、CI workflow、Prometheus/Grafana/OTel 配置。

## 3. 架构解释

### 目录结构

- `cmd/server`：后端入口。
- `internal/app`：依赖装配和生命周期。
- `internal/http`：router、中间件、健康检查和统一响应。
- `internal/module/*`：按业务职责拆分模块。
- `migrations`：数据库结构。
- `web`：前端工作台。
- `deployments`：本地和集群部署配置。

### 职责边界

- Handler 只处理 HTTP 参数、认证上下文和错误码。
- Service 处理业务规则。
- Repository 处理数据库访问。
- Collector 处理外部内容读取。
- Assistant 处理 AI provider 抽象。
- Observability 不嵌入业务逻辑，而是通过 middleware、observer 和 GORM callback 接入。

## 4. 核心流程解释

### 启动流程

`main()` 调用 `app.Run()`，读取配置后初始化 PostgreSQL、Redis、token manager、业务服务、Kafka 组件和观测组件；再根据 `app.mode` 启动 HTTP server、scheduler、worker 或全部。

### 登录流程

登录时 service 按 email 查用户，用 bcrypt 校验密码，生成 JWT access token 和随机 refresh token；refresh token 只保存 hash。受保护请求由 middleware 解析 Bearer token，把 userID 写入 request context。

### 采集流程

采集入口用 userID 和 sourceID 查 source，按 source type 找 collector，拿 Redis lock，创建 running collection run，采集上游内容，转成 article 并通过唯一索引去重入库，最后更新 run 和 source fetch 状态。

### 异步流程

Kafka 开启时，API 写 outbox 返回 queued。dispatcher 发送 Kafka。worker 消费任务并调用采集 service。失败时写 failed event，判断 retry 或 DLQ。

### AI 流程

摘要请求先入队，summary worker claim pending job 后调用 Assistant。embedding 是 hash token vector，存到 PostgreSQL JSONB。相似文章用 cosine similarity。RAG 搜索先查询文章，再返回本地生成的答案和 citations。

## 5. 高频问题

| Question | Expected Answer Outline | Source Evidence |
|---|---|---|
| 项目入口在哪里？ | `cmd/server/main.go` 只调用 `app.Run()`，装配在 `internal/app/server.go` | `cmd/server/main.go`, `internal/app/server.go` |
| 为什么要有 `app.mode`？ | 同一二进制可以作为 API、scheduler、worker 独立运行 | `internal/app/mode.go`, K8s deployments |
| 用户隔离如何实现？ | middleware 解析 token 得到 userID，repository 查询时按 userID 过滤 source/article | `middleware/auth.go`, `source/repository.go`, `article/repository.go` |
| source config 为什么要脱敏？ | 返回 API 前把 password/token/api_key 等 key 替换为 `[REDACTED]` | `source/service.go` |
| RSS 防 SSRF 怎么做？ | URL 校验 scheme、host，DialContext 禁止 loopback/private/link-local 等地址 | `internal/netguard/netguard.go`, `rss/collector.go` |
| 文章如何去重？ | `CreateIfNotExists` 使用 `OnConflict DoNothing`，数据库有 source+external_id 和 source+content_hash 唯一索引 | `article/repository.go`, migration 000001 |
| 同一个 source 并发采集怎么办？ | Redis `SETNX` source lock，释放时用 Lua 校验锁值 | `collector/lock.go` |
| Kafka outbox 解决什么问题？ | 先把待发送事件落库，再由 dispatcher 投递，降低 API 和 Kafka 之间失败丢任务风险 | `collectionjob/outbox.go` |
| retry 和 DLQ 如何判断？ | retryable 失败增加 attempt，永久错误或达到 max attempts 写 DLQ | `collectionjob/worker.go`, `errors.go` |
| AI 是真实 LLM 吗？ | 默认不是，默认 `ExtractiveAssistant` 是本地摘要和 hash embedding，可替换为真实 provider | `ai/assistant.go`, `ai/service.go` |
| RAG 如何实现？ | 先用文章全文搜索取候选，再用 Assistant 返回答案和 citations | `ai/service.go`, `article/repository.go` |
| 监控覆盖哪些？ | HTTP、collection、Kafka job、GORM metrics，tracing 可选 | `observability/metrics.go`, `tracing.go` |

## 6. 风险问题

| Question | Safe Answer |
|---|---|
| 这是生产系统吗？ | 这是具备生产形态组件的完整项目，但我不会声称它已经承载真实生产流量。 |
| Kafka 是否 exactly-once？ | 不是。它用 outbox、retry、DLQ、source lock 和数据库唯一索引降低失败与重复风险。 |
| AI 是否用了 OpenAI 或其他外部模型？ | 默认没有。代码通过 Assistant 接口预留扩展点，默认 provider 是本地算法。 |
| DLQ 管理是否安全？ | 当前看到的是登录态保护，没有看到 admin 权限和 user scope 过滤，这是一个后续需要加强的点。 |
| K8s 是否已经生产验证？ | manifests 和 overlay 存在，也有渲染校验脚本；本轮没有真实集群运行证据。 |

## 7. 不确定时的安全回答

- “我不想夸大这个点。源码里能证明的是……”
- “默认实现是……，如果要达到生产级，还需要补……”
- “这个流程的可靠性来自多个机制组合，不是单个机制保证。”
- “我可以按入口、数据表、service、repository 这条链路解释。”
- “我没有看到真实线上运行证据，所以只把它描述为具备部署清单和工程化配置。”

## 8. 不要说的内容

- “我接入了真实 LLM 并完成完整 RAG。”
- “系统是 exactly-once 消息处理。”
- “已经生产级部署并稳定运行。”
- “权限控制已经完整。”
- “所有功能都有端到端验证。”

## 9. 使用前需要再回源码确认的说法

- 任何具体性能指标。
- 任何真实部署规模。
- Email IMAP 在真实邮箱上的运行结果。
- Docker Compose 在当前机器上的实际启动结果。
- Kubernetes 在真实集群中的部署结果。
- 当前 `scripts/ci.sh` 的具体行为，因为该文件有未提交改动，本轮未读取其内容。

## 10. 最终面试安全总结

contentflow 可以安全定位为“一个工程化较完整的内容聚合系统项目”。最值得讲的不是某个单点功能，而是入口装配、用户认证、source 到 article 的核心业务流、Kafka 异步可靠性机制、Redis 缓存/限流/锁、AI provider 抽象和观测部署配置。讲的时候要主动降级 AI、RAG、生产级和 exactly-once 这些容易被夸大的点。
