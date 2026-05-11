# contentflow

contentflow 是一个仅后端的内容聚合系统，使用 Go 构建。

## 目标

- RSS 来源采集
- 邮件 newsletter 采集
- 文章去重
- 用户认证
- JWT 和刷新令牌
- Redis 缓存、锁和限流
- Kafka 异步采集流水线
- OpenAPI 契约
- 使用 gomock 和 testcontainers 进行测试
- 使用 Prometheus、Grafana 和 OpenTelemetry 实现可观测性
- Docker Compose 和 Kubernetes 部署

## 技术栈

- Go
- Gin
- PostgreSQL
- GORM
- Redis
- Kafka
- Viper
- slog
- golang-migrate
- OpenAPI
- Docker Compose
- Kubernetes
