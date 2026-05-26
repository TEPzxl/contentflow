# Stage 1: 入口、配置和运行模式

## 0. 术语预解释

- 二进制入口：操作系统启动程序时最先执行的代码位置。
- 配置：不写死在代码里的运行参数，例如端口、数据库地址、运行模式。
- 运行模式：同一份程序按不同职责启动，例如 API、scheduler、worker。
- 依赖装配：把数据库、缓存、service、handler 等对象创建并连接起来。

## 1. Learning Target

你需要能从 `go run ./cmd/server` 解释到 HTTP server、scheduler、worker 是如何被创建的。

## 2. Files to Read

- `cmd/server/main.go`
- `internal/app/mode.go`
- `internal/config/config.go`
- `internal/app/server.go`
- `configs/config.yaml`
- `configs/config.docker.yaml`

## 3. Reading Sequence

1. 先读 `cmd/server/main.go`，确认入口很薄。
2. 再读 `internal/app/mode.go`，理解 mode 到 runtime plan 的映射。
3. 再读 `internal/config/config.go`，看配置结构、默认值和环境变量覆盖。
4. 最后读 `internal/app/server.go`，只画装配顺序，不要陷入每个 service 细节。

## 4. Key Code Objects

| Name | Type | File | Why It Matters |
|---|---|---|---|
| `main` | function | `cmd/server/main.go` | 程序最先执行的位置 |
| `Run` | function | `internal/app/server.go` | 整个后端的装配和生命周期中心 |
| `runtimePlan` | struct | `internal/app/mode.go` | 表示是否启动 HTTP、scheduler、worker |
| `runtimePlanForMode` | function | `internal/app/mode.go` | 把 `app.mode` 转成运行计划 |
| `Config` | struct | `internal/config/config.go` | 项目所有配置的总结构 |

## 5. Hands-On Checks

```fish
go test ./internal/app ./internal/config
```

只验证启动模式和配置相关测试，不会修改业务代码。

## 6. Source Notes

- `main()` 只调用 `app.Run()`，错误时记录日志并退出。
- `app.mode=""` 或 `all` 会启动 HTTP、scheduler、worker。
- `api` 只启动 HTTP，`scheduler` 只启动定时器，`worker` 只启动 Kafka worker。
- `worker` 模式如果 Kafka 没启用会报错。
- 配置文件默认是 `configs/config.yaml`，也可以由 `CONTENTFLOW_CONFIG` 覆盖。

## 7. Diagram Task

画一个盒子图：

```text
cmd/server/main.go
  -> app.Run
    -> config.Load
    -> runtimePlanForMode
    -> NewPostgres / NewRedis
    -> NewService / NewHandler
    -> NewRouter
    -> HTTP server / scheduler / worker
```

## 8. Self-Test

1. `cmd/server/main.go` 为什么不直接创建 router？
2. `runtimePlanForMode("api")` 返回什么？
3. `runtimePlanForMode("bad")` 会怎样？
4. `CONTENTFLOW_CONFIG` 的作用是什么？
5. 本地配置和 Docker 配置的 Kafka 开关有什么不同？
6. 为什么 worker 模式必须要求 Kafka 开启？
7. 什么时候会注册 `/metrics`？
8. 生产环境弱 JWT secret 在哪里被拦截？

## 9. Interview Drill

### 30-second explanation

“项目入口是 `cmd/server/main.go`，真正的启动逻辑集中在 `app.Run()`。它读取配置、连接 PostgreSQL 和 Redis、创建所有业务模块，然后根据 `app.mode` 启动 API、scheduler、worker 或全部。”

### 2-minute explanation

“这个项目把启动装配放在 `internal/app/server.go`，这样业务模块不会自己创建数据库或 Redis。配置先通过 Viper 读取 YAML 和环境变量，再创建 logger、tracing、database、redis、metrics、认证、source、article、collector、Kafka 和 AI。Kafka 开关会改变采集 route 的实现，mode 决定当前进程跑 API、scheduler 还是 worker。”

### Follow-up questions

| Question | Answer Outline |
|---|---|
| 为什么要分 mode？ | 同一镜像可以按不同进程职责部署 |
| all 模式适合什么？ | 本地开发或简单单进程运行 |
| 生产为何拆 API/worker/scheduler？ | 隔离扩缩容和故障影响 |
| 配置错误会在哪里暴露？ | 启动期返回错误，main 记录并退出 |
| Docker 配置为什么启用 Kafka？ | Compose 启动了 Kafka，异步链路可用 |

## 10. Resume Risk Notes

本阶段只支持“理解和解释启动装配”，不能据此声明系统已生产部署。

## 11. Completion Checklist

- [ ] 能说出真实入口文件。
- [ ] 能解释四种 `app.mode`。
- [ ] 能画出 app 装配顺序。
- [ ] 能指出本地配置和 Docker 配置的主要差异。

