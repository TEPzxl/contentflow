# Troubleshooting Notes

## 后端启动失败

结论：先看配置路径和依赖连通性。

- 确认 `CONTENTFLOW_CONFIG` 是否指向存在的 YAML。
- 确认 PostgreSQL、Redis、Kafka 地址与端口。
- 本地 Compose 环境先跑 `docker compose -f deployments/docker-compose.yaml ps`。
- `/readyz` 返回 `503` 时，根据响应里的 dependency 名称定位。

## 登录失败

结论：区分凭据错误、JWT 配置错误和限流。

- `401 invalid_credentials`：邮箱或密码错误。
- `429 rate_limited`：Redis 限流生效，等待窗口结束。
- Refresh 失败时检查 `CONTENTFLOW_AUTH_JWT_SECRET` 是否在多副本间一致。

## 采集任务一直 queued

结论：优先检查 Kafka、outbox dispatcher 和 worker。

- 查看 `outbox_events` 是否有大量 `pending` / `failed`。
- 查看 worker 日志是否消费 `collection.requested`。
- Kafka 不可用时，先恢复 broker，再观察 outbox 是否继续 dispatch。

## DLQ 持续增长

结论：不要直接批量 replay，先按错误归因。

- 按 `error_message` 和 `source_id` 分组。
- 上游 RSS / IMAP 问题修复后再 replay。
- 永久错误使用 handled 标记，避免重复放大失败。

## AI 摘要不生成

结论：摘要请求只入队，实际生成由 AI summary worker 处理。

- 检查 `article_summaries.status` 是否为 `pending` 或 `failed`。
- 确认运行模式包含 `scheduler` 或 `worker`。
- 检查 worker 日志里的 `process ai summary failed`。
- 当前默认 Assistant 不依赖外部模型密钥；替换真实 LLM provider 后再检查密钥与网络。

## 前端无法访问 API

结论：先确认 `NEXT_PUBLIC_CONTENTFLOW_API_BASE_URL`。

- 本地开发默认应为 `http://localhost:8080/api/v1`。
- Compose 中前端服务使用 `http://localhost:8080/api/v1` 给浏览器访问。
- 如果后端返回 401，前端会尝试 refresh token；仍失败会清理本地 session。

## K8s 部署异常

结论：先渲染再 apply。

```fish
scripts/validate_k8s.sh
kubectl kustomize deployments/k8s/overlays/dev
```

- prod overlay 使用 ExternalSecret，并删除 base 中的静态 Secret。
- Ingress TLS 依赖 cert-manager issuer。
- migration job 需要在应用 rollout 前完成。
