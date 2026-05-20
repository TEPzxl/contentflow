# Load Testing Notes

## 目标

压测用于验证核心读路径、认证路径和采集触发路径的延迟边界，不替代容量规划。

## 覆盖接口

- `POST /api/v1/auth/login`
- `GET /api/v1/sources`
- `GET /api/v1/articles`
- `GET /api/v1/articles/{id}`
- `POST /api/v1/sources/{id}/collect`
- `GET /api/v1/sources/{id}/collection-runs`

## 运行方式

```fish
BASE_URL=http://localhost:8080 \
ACCESS_TOKEN=<token> \
EMAIL=demo@example.com \
PASSWORD=password123 \
SOURCE_ID=1 \
ARTICLE_ID=1 \
VUS=20 \
DURATION=1m \
k6 run scripts/load_articles.js
```

如果没有 `ACCESS_TOKEN`，脚本会尝试用 `EMAIL` / `PASSWORD` 登录。没有 `SOURCE_ID` 或 `ARTICLE_ID` 时，相关接口会跳过。

## 默认阈值

- `http_req_failed < 1%`
- `http_req_duration p95 < 500ms`

## 记录模板

```text
日期:
环境:
Git commit:
数据规模:
VUS / duration:

结果:
- http_req_failed:
- http_req_duration p50:
- http_req_duration p95:
- http_req_duration p99:
- RPS:

观察:
- 数据库:
- Redis:
- Kafka:
- API CPU / memory:

瓶颈:
-

优化记录:
-
```

## 已知优化点

- Article list 已接入 Redis 列表缓存，适合读多写少场景。
- Source collection 使用 Redis lock 避免同源并发采集。
- Kafka outbox 降低请求路径对 broker 可用性的敏感度。
- `/metrics` 可以配合 Grafana SLO dashboard 观察 5xx、p95 和 DB latency。
