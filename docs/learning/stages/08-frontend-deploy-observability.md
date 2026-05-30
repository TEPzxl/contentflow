# Stage 8: 前端、部署、监控和验证

## 0. 术语预解释

- 前端工作台：用户在浏览器里操作系统功能的页面。
- API client：前端封装 HTTP 请求的代码。
- Session：浏览器中保存的登录状态。
- Metrics：用数字形式记录系统运行状态。
- Tracing：记录一次请求经过哪些服务或组件的链路。
- CI：持续集成，代码提交后自动运行测试、构建和校验。
- Deployment：把应用以可运行方式部署到容器或集群。

## 1. Learning Target

你需要能解释用户如何通过前端调用 API，以及项目如何在本地、Docker、Kubernetes 和 CI 中运行或校验。

## 2. Files to Read

- `web/app/page.tsx`
- `web/features/workbench/workbench.tsx`
- `web/lib/api/client.ts`
- `web/lib/auth/session.ts`
- `web/package.json`
- `deployments/docker-compose.yaml`
- `deployments/k8s/base/backend-deployment.yaml`
- `deployments/k8s/base/worker-deployment.yaml`
- `deployments/k8s/base/scheduler-deployment.yaml`
- `internal/observability/metrics.go`
- `internal/observability/tracing.go`
- `.github/workflows/ci.yaml`

## 3. Reading Sequence

1. `web/app/page.tsx` 看前端入口。
2. `workbench.tsx` 看页面状态如何组织。
3. `api/client.ts` 看 API 调用和 token refresh。
4. `session.ts` 看 access token 存储。
5. Docker Compose 看本地服务组合。
6. K8s deployments 看 API/worker/scheduler 拆分。
7. metrics/tracing 看观测入口。
8. CI workflow 看验证类别。

## 4. Key Code Objects

| Name | Type | File | Why It Matters |
|---|---|---|---|
| `HomePage` | function | `web/app/page.tsx` | 前端页面入口 |
| `Workbench` | component | `web/features/workbench/workbench.tsx` | 前端主工作台 |
| `api` | object | `web/lib/api/client.ts` | 前端所有后端调用集中处 |
| `readSession` | function | `web/lib/auth/session.ts` | 登录状态读取 |
| `Metrics` | struct | `internal/observability/metrics.go` | Prometheus 指标定义 |
| `InitTracing` | function | `internal/observability/tracing.go` | OpenTelemetry 初始化 |

## 5. Hands-On Checks

```fish
npm --prefix web run typecheck
npm --prefix web run lint
go test ./internal/observability ./internal/http
```

完整本地栈只在需要时启动：

```fish
docker compose -f deployments/docker-compose.yaml up --build
```

## 6. Source Notes

- `HomePage` 直接渲染 `Workbench`，没有营销页。
- `Workbench` 登录后显示文章、来源、采集记录、AI、设置等视图。
- access token 保存在 `sessionStorage`。
- API client 默认带 Bearer token，遇到 401 会尝试 refresh。
- Compose 中前端映射到 `3001:3000`，Grafana 用 `3000`。
- K8s backend 使用 `CONTENTFLOW_APP_MODE=api`。
- K8s worker 使用 `CONTENTFLOW_APP_MODE=worker`。
- K8s scheduler 使用 `CONTENTFLOW_APP_MODE=scheduler`。
- metrics 包含 HTTP、collection、Kafka job、GORM operation。

## 7. Diagram Task

```text
Browser
  -> Workbench
  -> api client
  -> Gin API
  -> auth/source/article/collector/ai
  -> PostgreSQL / Redis / Kafka
  -> Prometheus / Grafana / OTel
```

## 8. Self-Test

1. 前端未登录时渲染什么？
2. access token 存在哪里？
3. refresh 请求为什么设置 `auth: false`？
4. `api.collectSource` 调哪个后端 route？
5. Compose 为什么有 migrate service？
6. backend、worker、scheduler 的 K8s mode 分别是什么？
7. `/metrics` 是在哪里注册的？
8. GORM metrics 通过什么机制接入？
9. tracing 关闭时 `InitTracing` 返回什么？
10. CI workflow 大致分哪些 job？

## 9. Interview Drill

### 30-second explanation

“前端是一个 Next.js 工作台，登录后通过统一 API client 调后端。access token 放在 sessionStorage，401 时尝试 refresh。部署上有 Docker Compose 本地栈，也有 K8s backend/worker/scheduler 拆分。观测上有 Prometheus metrics、OpenTelemetry tracing 和 Grafana 配置。”

### 2-minute explanation

“前端入口是 `web/app/page.tsx`，主组件 `Workbench` 组织文章、来源、采集记录和 AI 面板。API client 负责统一加 Bearer token、处理 refresh 和错误文案。后端部署支持 Compose 一键启动完整依赖，也支持 K8s 用同一镜像按 mode 拆成 API、worker、scheduler。metrics 通过 Gin middleware、collection observer、Kafka job observer 和 GORM callbacks 接入，tracing 通过 OpenTelemetry OTLP exporter 接入。”

### Follow-up questions

| Question | Answer Outline |
|---|---|
| 为什么前端端口是 3001？ | 避免和 Grafana 3000 冲突 |
| access token 放 sessionStorage 有什么取舍？ | 简单；刷新后会丢登录态，XSS 风险仍需考虑 |
| Compose 适合什么？ | 本地开发和集成验证 |
| K8s HPA 覆盖谁？ | backend deployment |
| CI 能证明生产可用吗？ | 不能，只能证明自动化校验通过 |

## 10. Resume Risk Notes

可以说“提供 Next.js 工作台、Docker Compose、本地监控和 Kubernetes manifests”，不要说“已经完成生产部署验证”。

## 11. Completion Checklist

- [ ] 能解释前端登录状态。
- [ ] 能解释 API client refresh 逻辑。
- [ ] 能解释 Compose 服务组成。
- [ ] 能解释 K8s mode 拆分。
- [ ] 能解释 metrics/tracing 入口。
