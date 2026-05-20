# Resume Notes

## 一句话项目描述

contentflow 是一个 Go + Next.js 内容聚合系统，支持 RSS / Email 采集、异步任务可靠性、文章查询与 AI 摘要/RAG，并具备 Docker Compose、Kubernetes、CI/CD 和可观测性闭环。

## 简历 Bullet Points

- 使用 Go、Gin、GORM 和 PostgreSQL 设计内容聚合后端，完成用户认证、内容源管理、文章入库去重、查询和状态管理等核心模块。
- 基于 Redis 实现文章列表缓存、登录/采集限流和采集分布式锁，降低重复采集和热点查询压力。
- 使用 Kafka + outbox pattern 构建异步采集流水线，实现任务重试、退避、DLQ、replay 和 handled 管理，提升采集链路可靠性。
- 为 RSS 与 Email 采集设计插件化 Collector 抽象，统一 collection run 记录、错误处理和指标观测。
- 使用 Prometheus、Grafana、OpenTelemetry 和 Jaeger 建立 API 延迟、错误率、采集失败、Kafka 任务和数据库延迟的观测闭环。
- 使用 React、Next.js、TypeScript、Tailwind CSS 构建前端工作台，覆盖认证、source 管理、文章阅读、采集记录和 AI 工作区。
- 设计 AI Assistant 抽象，在文章数据上实现异步摘要、embedding、相似文章、Daily Digest 和 RAG 搜索，记录 model、prompt version 和结果状态。
- 建立 Go test、gomock、testcontainers、Playwright、OpenAPI 校验、K8s 渲染校验和 GitHub Actions CI，提升交付可靠性。
- 使用 Docker Compose 和 Kubernetes Kustomize overlays 支持本地、dev、staging、prod 环境，配套 TLS ingress、External Secrets 和镜像发布 workflow。

## 面试讲解结构

1. 业务背景：聚合多来源内容，解决采集、去重、阅读和后续 AI 增强。
2. 模块拆分：auth、source、collector、article、collectionjob、ai。
3. 可靠性：outbox、retry backoff、DLQ、idempotency key。
4. 性能：Redis cache、限流、锁、分页查询、k6 压测。
5. 可观测性：metrics、dashboard、alerts、tracing、runbook。
6. 部署：Compose 本地闭环，K8s overlays 区分环境，GHCR 发布镜像。
7. AI：异步摘要和可替换 provider，避免同步 HTTP 主链路调用外部模型。
