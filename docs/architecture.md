# Contentflow Architecture

## 职责边界

```text
cmd/server                 程序入口
internal/app               依赖组装、生命周期、后台 worker
internal/http              Router、middleware、统一响应
internal/module/auth       用户认证、JWT、Refresh Token
internal/module/source     内容源管理
internal/module/collector  RSS / Email 采集编排、collection run
internal/module/article    文章入库、查询、状态、缓存
internal/module/collectionjob Kafka 任务、outbox、retry、DLQ
internal/module/ai         摘要、embedding、Digest、RAG
internal/observability     metrics、tracing、GORM callbacks
deployments                Docker、Prometheus、Grafana、K8s
web                        Next.js 前端工作台
```

## 系统总览

```mermaid
flowchart TD
  Browser[Browser] --> Web[Next.js Frontend]
  Web --> API[Gin API]
  API --> Auth[Auth Module]
  API --> Source[Source Module]
  API --> Article[Article Module]
  API --> AI[AI Module]
  API --> Redis[(Redis)]
  Auth --> PG[(PostgreSQL)]
  Source --> PG
  Article --> PG
  AI --> PG
  Scheduler[Scheduler] --> Collector[Collector Service]
  Collector --> RSS[RSS Feed]
  Collector --> Email[Email Mailbox]
  Collector --> PG
```

## RSS / Email 采集数据流

```mermaid
sequenceDiagram
  participant User
  participant API
  participant Collector
  participant SourceRepo
  participant ArticleRepo
  participant Upstream as RSS/Email

  User->>API: POST /sources/{id}/collect
  API->>Collector: CollectSource(userID, sourceID)
  Collector->>SourceRepo: Find source with user scope
  Collector->>Upstream: Fetch feed/mailbox
  Upstream-->>Collector: Items
  Collector->>ArticleRepo: CreateIfNotExists by source/content hash
  Collector-->>API: Collection run result
```

## Kafka 异步任务流

```mermaid
sequenceDiagram
  participant API
  participant Outbox
  participant Kafka
  participant Worker
  participant DLQ

  API->>Outbox: Persist collection.requested
  Outbox->>Kafka: Dispatch event
  Kafka->>Worker: Consume collection.requested
  Worker->>Worker: Collect source
  alt success
    Worker->>Kafka: collection.completed
  else retryable failure
    Worker->>Kafka: collection.failed
    Worker->>Kafka: collection.requested with next_attempt_at
  else permanent/max attempts
    Worker->>DLQ: Persist failed payload
    Worker->>Kafka: collection.dlq
  end
```

## 前后端交互

```mermaid
flowchart LR
  Web[Next.js Workbench] --> AuthAPI[/Auth APIs/]
  Web --> SourceAPI[/Source APIs/]
  Web --> ArticleAPI[/Article APIs/]
  Web --> CollectAPI[/Collection APIs/]
  Web --> AIAPI[/AI APIs/]
  AuthAPI --> JWT[Access Token + Refresh Token]
```

## AI 任务流

```mermaid
sequenceDiagram
  participant Web
  participant API
  participant DB
  participant Worker
  participant Assistant

  Web->>API: POST /articles/{id}/summary
  API->>DB: upsert article_summaries pending
  Worker->>DB: claim pending summary
  Worker->>Assistant: Summarize(article)
  Assistant-->>Worker: summary + model + prompt_version
  Worker->>DB: mark succeeded
  Web->>API: GET /articles/{id}/summary
  API-->>Web: summary status/result
```

AI 模块通过 `Assistant` 接口隔离模型提供方。当前默认实现是可预测的本地 extractive/hash 算法，便于测试和无外部密钥运行；后续可以替换成真实 LLM / embedding provider。

## 部署拓扑

```mermaid
flowchart TD
  Ingress[Ingress TLS] --> Frontend[frontend Deployment]
  Ingress --> Backend[backend Deployment]
  Backend --> Postgres[(PostgreSQL)]
  Backend --> Redis[(Redis)]
  Backend --> Kafka[(Kafka)]
  Scheduler[scheduler Deployment] --> Kafka
  Worker[worker Deployment] --> Kafka
  Worker --> Postgres
  Prometheus[Prometheus] --> Backend
  Grafana[Grafana] --> Prometheus
  Backend --> OTel[OTel Collector]
  OTel --> Jaeger[Jaeger]
  ExternalSecret[ExternalSecret] --> Secret[K8s Secret]
  Secret --> Backend
```
