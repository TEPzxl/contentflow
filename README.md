# contentflow

contentflow 当前主体是 Go 后端内容聚合系统，后续规划使用 React + Next.js + TypeScript + Tailwind CSS 补齐前端应用。

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
- React
- Next.js
- TypeScript
- Tailwind CSS

## 文档

- OpenAPI 契约：`api/openapi.yaml`
- 本地 Docker Compose：`deployments/docker-compose.yaml`
- Kubernetes 配置：`deployments/k8s/`
- 前端应用：`web/`

## 项目结构

下面是到目录层级的项目结构（仅列出目录，不包含具体文件）：

```contentflow/README.md#L1-200
contentflow/
├── cmd/ # 负责程序入口
│   └── server/ 
├── configs/ # 配置文件
├── internal/
│   ├── app/ # 应用组装与生命周期管理
│   ├── config/ # 配置
│   ├── logger/ # 日志记录
│   ├── database/ # 数据库
│   ├── cache/ # 缓存
│   ├── http/ # HTTP服务
│   │   └── handler/
│   └── module/
│       ├── auth/
│       ├── user/
│       ├── source/
│       ├── article/
│       └── collector/
├── migrations/
├── api/ # OpenAPI 契约
├── deployments/ # 部署配置
├── scripts/ # 部署脚本
├── tests/ # 测试
└── web/ # Next.js 前端应用
```
