# 项目运行手册

## 0. 术语预解释

- 运行手册：把项目启动、测试、构建、排障相关命令集中记录的文档。
- 环境要求：运行项目之前本机需要具备的软件。
- migration：数据库结构变更脚本。
- Compose：Docker Compose，用一个 YAML 文件启动多个本地容器。
- Kustomize：Kubernetes 原生配置组合工具，用 base 和 overlays 生成最终 YAML。
- 已验证：本轮实际运行并确认成功。
- 未验证：仓库中有命令或配置，但本轮没有执行。

## 1. Environment Requirements

从仓库文件可见的要求：

- Go：`go.mod` 声明 `go 1.25.0`。
- Node.js：`.github/workflows/ci.yaml` 使用 `NODE_VERSION: "22"`。
- npm：`web/package.json` 使用 npm scripts。
- Docker：`deployments/docker-compose.yaml` 和 Dockerfiles 需要 Docker。
- Docker Compose：README 和 Makefile 都使用 `docker compose`。
- PostgreSQL：后端运行需要连接 PostgreSQL。
- Redis：后端启动会 ping Redis。
- Kafka：异步采集模式需要 Kafka；本地 `configs/config.yaml` 默认关闭，`configs/config.docker.yaml` 和 K8s 配置开启。
- migrate CLI：Makefile 的 `migrate-up` 需要 `migrate` 命令。
- kubectl：`scripts/validate_k8s.sh` 使用 `kubectl kustomize`。

## 2. Setup Commands

```fish
npm --prefix web install
```

状态：未验证。

用途：安装前端依赖。

```fish
go mod download
```

状态：未验证。

用途：下载 Go 依赖。

## 3. Start Commands

```fish
go run ./cmd/server
```

状态：未验证。

用途：在本机直接启动后端。需要本机已有 PostgreSQL 和 Redis，并且配置文件可连接。

```fish
CONTENTFLOW_CONFIG=configs/config.yaml go run ./cmd/server
```

状态：未验证。

用途：显式指定本地配置启动后端。

```fish
docker compose -f deployments/docker-compose.yaml up --build
```

状态：未验证。

用途：启动完整本地栈，包括 backend、frontend、postgres、redis、kafka、migration、observability 组件。

```fish
npm --prefix web run dev
```

状态：未验证。

用途：启动前端开发服务器。

## 4. Test Commands

```fish
go test ./...
```

状态：未验证。

用途：运行 Go 测试。注意当前工作区已有用户未提交改动，若失败需要先区分是现有改动还是项目原问题。

```fish
go test ./internal/app ./internal/config
```

状态：未验证。

用途：验证启动模式和配置相关测试。

```fish
go test ./internal/module/auth/... ./internal/module/source/... ./internal/module/collector/... ./internal/module/article/... ./internal/module/collectionjob/... ./internal/module/ai/...
```

状态：未验证。

用途：按主要业务模块运行后端测试。

```fish
npm --prefix web run typecheck
```

状态：未验证。

用途：运行前端 TypeScript 类型检查。

```fish
npm --prefix web run lint
```

状态：未验证。

用途：运行前端 ESLint。

```fish
npm --prefix web run build
```

状态：未验证。

用途：构建前端。

```fish
npm --prefix web run test
```

状态：未验证。

用途：运行 Playwright 测试。可能需要先安装浏览器依赖。

## 5. Lint / Format Commands

```fish
go fmt ./...
```

状态：未验证。

用途：格式化 Go 代码。本轮禁止修改业务代码，所以不执行。

```fish
go vet ./...
```

状态：未验证。

用途：运行 Go 静态检查。

```fish
npm --prefix web run lint
```

状态：未验证。

用途：前端 lint。

## 6. Database / Migration Commands

Makefile 中定义：

```fish
make migrate-up
```

状态：未验证。

用途：执行 `migrations` 下的 up migration。

```fish
make migrate-down
```

状态：未验证。

用途：回滚一个 migration。会改变数据库结构，运行前需要确认。

```fish
make migrate-version
```

状态：未验证。

用途：查看当前数据库 migration 版本。

```fish
make migrate-force VERSION=1
```

状态：未验证。

用途：强制设置 migration 版本。会改变 migration 状态，除非明确需要，不建议学习阶段运行。

## 7. Docker / Compose Commands

Makefile 中定义：

```fish
make compose-up
make compose-ps
make compose-logs
make compose-down
```

状态：未验证。

用途：

- `compose-up`：构建并启动完整容器栈。
- `compose-ps`：查看容器状态。
- `compose-logs`：查看日志。
- `compose-down`：停止并删除 Compose 创建的容器网络。

`scripts/dev_stack.sh` 也支持：

```fish
scripts/dev_stack.sh up
scripts/dev_stack.sh logs backend
scripts/dev_stack.sh down
```

状态：未验证。

## 8. Kubernetes Commands

```fish
scripts/validate_k8s.sh
```

状态：未验证。

用途：使用 `kubectl kustomize` 渲染 base/dev/staging/prod，并检查关键资源。

```fish
kubectl kustomize deployments/k8s/base
```

状态：未验证。

用途：渲染 Kubernetes base manifests。

```fish
kubectl kustomize deployments/k8s/overlays/prod
```

状态：未验证。

用途：渲染 prod overlay。

## 9. Common Failures

| Symptom | Likely Cause | What to Check |
|---|---|---|
| 后端启动失败 `ping postgres` | PostgreSQL 未运行或配置不匹配 | `configs/config.yaml` database 部分 |
| 后端启动失败 `ping redis` | Redis 未运行或地址不对 | `configs/config.yaml` redis 部分 |
| `app mode worker requires kafka.enabled=true` | worker 模式但 Kafka 关闭 | `app.mode` 和 `kafka.enabled` |
| 登录一直 401 | access token 缺失、过期或 JWT secret 不一致 | `Authorization` header、auth config |
| RSS source 创建失败 | URL 不安全或不是 HTTP/HTTPS | `netguard.ValidateHTTPURL` 规则 |
| 采集返回 `collection_in_progress` | 同 source Redis lock 已存在 | `collection:lock:source:<id>` |
| DLQ replay 后仍失败 | 原 source 或上游问题仍存在 | DLQ item payload、source config、worker logs |
| K8s 校验失败 | 缺 `kubectl` 或 manifests 不可渲染 | `scripts/validate_k8s.sh` |
| 前端 API 请求失败 | API base URL 配置错误或后端未启动 | `NEXT_PUBLIC_CONTENTFLOW_API_BASE_URL` |

## 10. What Each Command Proves

| Command | Proves |
|---|---|
| `go test ./internal/app ./internal/config` | 运行模式和配置安全校验基本正确 |
| `go test ./internal/module/auth/...` | 认证 service、handler、token 行为有测试覆盖 |
| `go test ./internal/module/collector/...` | 采集编排、RSS/Email、lock、scheduler 相关行为有测试覆盖 |
| `go test ./internal/module/collectionjob` | outbox、worker、retry、DLQ 行为有测试覆盖 |
| `go test ./internal/module/ai` | AI service、summary、embedding、RAG 行为有测试覆盖 |
| `go test ./api` | OpenAPI 文档结构可被测试解析 |
| `npm --prefix web run typecheck` | 前端 TypeScript 类型一致 |
| `npm --prefix web run build` | 前端可构建 |
| `docker compose -f deployments/docker-compose.yaml up --build` | 本机完整容器栈能启动 |
| `scripts/validate_k8s.sh` | Kubernetes manifests 可被 kustomize 渲染并包含关键资源 |

## 11. 本轮验证状态

本轮主要任务是生成学习文档，且你明确禁止修改业务代码。我没有运行会格式化或改变业务代码的命令，也没有启动会写入数据库或创建容器的命令。

由于 `internal/module/collector/rss_integration_test.go` 和 `scripts/ci.sh` 已有用户未提交改动，本轮也没有基于它们的当前内容下结论。
