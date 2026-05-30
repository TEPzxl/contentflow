# Stage 6: Kafka outbox、worker、retry 和 DLQ

## 0. 术语预解释

- Kafka：一种消息队列系统，用 topic 保存事件，worker 可以异步消费。
- Outbox：先把待发送消息写进数据库，再由后台 dispatcher 投递，降低丢消息风险。
- Worker：后台消费者进程，负责处理队列里的任务。
- Retry：失败后再次尝试。
- DLQ：Dead Letter Queue，多次失败后保存下来等待人工处理或 replay。
- Replay：把失败任务重新投回队列。

## 1. Learning Target

你需要能解释异步采集如何进入 outbox、投递 Kafka、被 worker 执行、失败重试和进入 DLQ。

## 2. Files to Read

- `internal/module/collector/async_handler.go`
- `internal/module/collectionjob/event.go`
- `internal/module/collectionjob/producer.go`
- `internal/module/collectionjob/outbox.go`
- `internal/module/collectionjob/outbox_repository.go`
- `internal/module/collectionjob/worker.go`
- `internal/module/collectionjob/errors.go`
- `internal/module/collectionjob/dlq.go`
- `internal/module/collectionjob/dlq_handler.go`
- `migrations/000002_create_kafka_reliability_tables.up.sql`

## 3. Reading Sequence

1. 先看 event struct 和 topic 常量。
2. 看 async handler 返回 202。
3. 看 outbox producer 如何创建 pending event。
4. 看 dispatcher 如何发送并标记 sent/failed。
5. 看 worker 如何处理成功、失败、重试。
6. 看 DLQ service 的 list/replay/handled。
7. 对照 migration。

## 4. Key Code Objects

| Name | Type | File | Why It Matters |
|---|---|---|---|
| `CollectionRequested` | struct | `event.go` | 异步采集请求消息 |
| `OutboxProducer` | struct | `outbox.go` | API 写 outbox 的入口 |
| `OutboxDispatcher` | struct | `outbox.go` | 把 outbox 投递到 Kafka |
| `Worker` | struct | `worker.go` | 消费采集任务并处理结果 |
| `IsPermanentError` | function | `errors.go` | 判断是否直接进 DLQ |
| `DLQService` | struct | `dlq.go` | 管理失败任务 |

## 5. Hands-On Checks

```fish
go test ./internal/module/collectionjob
```

## 6. Source Notes

- `kafka.enabled=true` 时 collect route 使用 async handler。
- API 返回 queued，不立即返回 collection run。
- outbox event 初始状态是 `pending`。
- dispatcher 对 `pending` 和 `failed` 且到期的 event 做投递。
- worker 成功后写 `collection.completed`。
- worker 失败后写 `collection.failed`。
- retryable failure 会重新写 `collection.requested`。
- permanent failure 或达到最大次数会写 DLQ。

## 7. Diagram Task

```text
POST /sources/:id/collect
  -> AsyncHandler.RequestCollection
  -> OutboxProducer.RequestCollection
  -> outbox_events
  -> OutboxDispatcher.DispatchReady
  -> Kafka collection.requested
  -> Worker.HandleMessage
  -> CollectionService.CollectSource
  -> completed / failed / retry / dlq
```

## 8. Self-Test

1. 为什么不直接从 API 写 Kafka？
2. outbox event 的状态有哪些？
3. worker 何时写 completed event？
4. retry 的 `NextAttemptAt` 如何计算？
5. permanent error 包括哪些？
6. DLQ replay 如何重置 event？
7. `idempotency_key` 当前在哪里使用？
8. 为什么不能说 exactly-once？
9. DLQ handler 当前权限边界有什么风险？
10. outbox dispatcher 写 Kafka 失败后如何处理？

## 9. Interview Drill

### 30-second explanation

“Kafka 开启后，采集请求先写 outbox，dispatcher 再投递 Kafka。worker 消费 `collection.requested` 后调用同一个采集 service。成功写 completed，失败写 failed，然后按 attempt 和 permanent error 判断 retry 或 DLQ。”

### 2-minute explanation

“outbox 用数据库保存待发送事件，解决 API 请求成功但 Kafka 写入失败的窗口。dispatcher 定期扫描 ready event 并发送。worker 的 `HandleMessage` 是状态机核心：解析请求、等待下次尝试时间、执行采集、写结果事件。失败时，retryable 错误会增加 attempt 并重新投递 requested；source 不存在或 collector 不存在等永久错误，或者达到最大次数，就写入 DLQ。DLQ 支持 list、replay 和 handled，并按登录用户过滤；如果要做后台运维，还需要补 admin/role 权限模型。”

### Follow-up questions

| Question | Answer Outline |
|---|---|
| outbox 的代价是什么？ | 多一张表和 dispatcher，延迟增加 |
| retry 如何避免无限重试？ | worker 有 max attempts；outbox dispatcher 自身失败目前没有最大次数 |
| DLQ 用来解决什么？ | 保存不可自动恢复的失败，避免任务无限卡住 |
| 幂等由什么组成？ | Kafka key、source lock、article unique index |
| 当前需要改进哪里？ | DLQ admin/role 权限、outbox 幂等唯一键、更多事务边界 |

## 10. Resume Risk Notes

可以说“实现了 outbox/retry/DLQ 可靠性机制”，不要说“Kafka exactly-once”。

## 11. Completion Checklist

- [ ] 能画出 outbox 到 worker 的流程。
- [ ] 能解释 retry 和 DLQ。
- [ ] 能解释为什么不能夸大幂等。
- [ ] 能指出 DLQ 只有用户隔离、没有 admin/role 权限模型。
