# 项目边界

## 0. 术语预解释

- 边界：项目实际负责什么、不负责什么，以及哪些说法有源码证据。
- 生产就绪：一个系统可以被长期稳定、安全、可观测、可运维地运行在真实用户环境里的程度。
- 持久化：把数据保存到数据库或其他存储，使进程重启后数据仍然存在。
- 可观测性：通过日志、指标和链路追踪了解系统运行状态。
- 扩展性：系统在请求量或数据量增长时继续工作的能力。
- 一致性：多次写入、失败重试或并发执行后，数据仍符合预期约束的能力。

## 1. 这个项目是什么

- 一个内容聚合 Web 应用：后端提供 HTTP API，前端提供工作台。
- 一个模块化 Go 后端项目：`internal/app` 负责组装，业务模块在 `internal/module/*`。
- 一个多源采集系统：支持 `rss` 和 `email` source type。
- 一个异步任务示例：Kafka outbox、worker retry、DLQ replay / handled 都有代码。
- 一个可观测性示例：Prometheus metrics、OpenTelemetry tracing、Grafana dashboard 配置存在。
- 一个可部署工程：Docker Compose 和 Kubernetes manifests 存在。
- 一个面试可讲的综合项目：入口、认证、数据建模、采集、异步、缓存、监控、前端都有可读代码路径。

## 2. 这个项目不是什么

- 不是已经证明承载真实用户流量的线上系统。
- 不是完整权限平台：没有 role、organization、admin permission 等模型。
- 不是完整大模型应用：默认 AI provider 是本地 `ExtractiveAssistant`，不是外部 LLM。
- 不是完整向量数据库 RAG：embedding 存在 PostgreSQL JSONB 中，相似度在 Go 内存中计算。
- 不是严格 exactly-once 消息系统：使用 outbox、retry、DLQ、锁和唯一索引缓解重复与失败，但不能夸大成严格一次处理。
- 不是完整邮件产品：Email collector 支持 IMAP 和目录读取路径，但本轮没有运行真实邮箱采集。

## 3. 生产就绪评估

| Area | Assessment | Evidence |
|---|---|---|
| Persistence | 较完整 | PostgreSQL migrations 覆盖用户、token、source、article、run、outbox、DLQ、AI 表 |
| Configuration | 较完整 | `configs/config.yaml`、`configs/config.docker.yaml`、K8s `configmap.yaml`，Viper 支持环境变量覆盖 |
| Deployment | 中等 | Docker Compose、Dockerfile、K8s base/overlays、release workflow 存在；未验证真实集群运行 |
| Observability | 中等偏强 | `/metrics`、HTTP metrics、GORM callbacks、collection/kafka metrics、OpenTelemetry tracing |
| Tests | 中等偏强 | 多个模块有 unit test、integration test、benchmark、OpenAPI test、Playwright E2E 文件 |
| Error handling | 中等 | handler 层有错误映射，service/repository 用 sentinel errors 和 wrapped errors |
| Security | 中等 | 密码 hash、JWT、refresh token hash、source config redaction、netguard、防 SSRF；DLQ 管理接口缺少更细权限边界 |
| Scalability | 中等 | API、scheduler、worker 可拆模式；K8s HPA 只覆盖 backend；数据库和 Kafka 扩展未深入实现 |
| Data consistency | 中等 | 文章去重靠唯一索引，collection lock 防并发采集，outbox 保留待发送事件；仍需更多幂等和事务边界说明 |
| Operational maturity | 中等 | runbook、troubleshooting、alert rules、dashboards 存在；缺少真实事故演练和部署结果证据 |

## 4. 安全可用的项目说法

- “实现了一个 Go + Next.js 的内容聚合系统，支持用户登录、source 管理、RSS / Email 采集、文章查询和状态管理。”
- “后端使用模块化结构，`internal/app` 统一装配数据库、Redis、Kafka、HTTP 路由、scheduler 和 worker。”
- “采集链路会记录 collection run，并通过文章唯一索引区分新增和重复。”
- “Kafka 模式下，采集请求先进入 outbox，再由 dispatcher 发往 Kafka，worker 消费后成功写 completed，失败写 failed、retry 或 DLQ。”
- “AI 模块通过 Assistant 接口隔离 provider，默认是可测试的本地实现。”
- “项目有 Prometheus metrics、OpenTelemetry tracing、Grafana dashboard、Docker Compose 和 Kubernetes manifests。”

## 5. 风险说法

- “生产级”：需要补充真实部署、稳定性、监控告警、故障恢复证据。
- “RAG”：需要解释这是 RAG 风格接口，不是外部 LLM + 向量数据库完整架构。
- “可靠消息”：可以讲 outbox/retry/DLQ，但不能讲 exactly-once。
- “安全”：可以讲已做密码 hash、token、netguard、secret 校验，但不能说安全体系完整。
- “Email”：可以讲代码支持 IMAP provider，但真实邮箱配置和运行未验证。

## 6. 应避免的说法

- “已经上线并处理大量真实用户内容。”
- “使用真实大模型完成摘要和问答。”
- “消息处理完全不会重复。”
- “DLQ 管理已经做了管理员权限控制。”
- “Kubernetes 生产部署已经验证无误。”

## 7. 建议项目定位

### Conservative version

这是一个围绕内容聚合的全栈学习项目，重点展示 Go 后端模块化、数据库建模、认证、采集、文章查询、异步任务和前端工作台。

### Balanced version

这是一个工程化较完整的内容聚合系统，包含 Go API、Next.js 工作台、PostgreSQL、Redis、Kafka outbox/retry/DLQ、Prometheus/OpenTelemetry 观测和 Docker/Kubernetes 部署配置；默认 AI provider 是本地算法实现，适合作为后续接入真实 provider 的扩展点。

### Strong but still defensible version

这是一个具备生产形态设计元素的内容聚合平台原型：后端按 API、scheduler、worker 模式拆分，采集链路支持同步和 Kafka 异步处理，数据层通过迁移和唯一约束维护核心一致性，运维层包含 metrics、tracing、dashboards、CI、容器化和 Kubernetes manifests；项目仍需要真实环境验证、权限细化和外部 AI provider 才能称为完整生产系统。
