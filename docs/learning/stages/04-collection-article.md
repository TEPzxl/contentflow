# Stage 4: 同步采集和文章入库

## 0. 术语预解释

- 采集：从 RSS 或 Email 等外部来源读取内容。
- 去重：避免同一篇文章被重复保存。
- Run：一次采集执行记录。
- Registry：按类型保存 collector 的注册表。
- 上游：被本系统访问的数据来源，例如 RSS feed 或邮箱。

## 1. Learning Target

你需要能完整解释一次手动采集如何从 source 变成 article。

## 2. Files to Read

- `internal/module/collector/collector.go`
- `internal/module/collector/registry.go`
- `internal/module/collector/service.go`
- `internal/module/collector/run_repository.go`
- `internal/module/collector/lock.go`
- `internal/module/collector/rss/collector.go`
- `internal/module/collector/email/collector.go`
- `internal/module/article/service.go`
- `internal/module/article/repository.go`
- `internal/module/article/model.go`

## 3. Reading Sequence

1. `Collector` interface。
2. `Registry.Get`。
3. `CollectionService.CollectSource`。
4. `rss.Collector.Collect`。
5. `email.Collector.Collect`。
6. `article.Service.SaveCollectedItems`。
7. `article.Repository.CreateIfNotExists`。

## 4. Key Code Objects

| Name | Type | File | Why It Matters |
|---|---|---|---|
| `Collector` | interface | `collector/collector.go` | 新采集类型的统一契约 |
| `CollectionService` | struct | `collector/service.go` | 采集编排核心 |
| `CollectionRun` | struct | `collector/model.go` | 记录一次采集结果 |
| `RedisCollectionLock` | struct | `collector/lock.go` | 防同 source 并发采集 |
| `CollectedItem` | struct | `collector/item.go` | collector 和 article service 的交界数据 |
| `CreateIfNotExists` | method | `article/repository.go` | 数据库去重入库 |

## 5. Hands-On Checks

```fish
go test ./internal/module/collector/... ./internal/module/article/...
```

## 6. Source Notes

- `CollectSource` 先确认 source 属于当前 user。
- `Registry` 根据 source type 找 RSS 或 Email collector。
- 采集前拿 Redis lock，没拿到就返回 `ErrCollectionInProgress`。
- run 先以 running 创建，完成后标记 success 或 failed。
- RSS 使用 HTTP fetch + gofeed parse。
- Email 支持 empty、directory、imap provider。
- article 入库用 `OnConflict DoNothing` 判断新增或重复。

## 7. Diagram Task

```text
POST /sources/:id/collect
  -> Handler.CollectSource
  -> CollectionService.CollectSource
  -> sourceRepo.FindByUserIDAndID
  -> registry.Get
  -> lock.Acquire
  -> runRepo.Create
  -> collector.Collect
  -> articleWriter.SaveCollectedItems
  -> runRepo.Finish
  -> sourceRepo.Update
```

## 8. Self-Test

1. `Collector` interface 为什么只需要 `Type` 和 `Collect`？
2. 采集前为什么要创建 run？
3. 采集失败时 run 如何结束？
4. RSS item 如何生成 content hash？
5. Email collector 如何跳过已见消息？
6. 文章唯一索引有哪些？
7. `DuplicatedCount` 从哪里来？
8. source 的 `LastFetchStatus` 在哪里更新？
9. 为什么要用 Redis lock 而不是只靠数据库唯一索引？
10. handler 为什么对 `ErrCollectionFailed` 仍返回 run 数据？

## 9. Interview Drill

### 30-second explanation

“采集服务先用 userID 和 sourceID 找 source，再按 source type 找 collector，获取 Redis 锁，创建 collection run，采集 RSS 或 Email，转换成 collected item，交给 article service 通过唯一索引去重入库，最后更新 run 和 source fetch 状态。”

### 2-minute explanation

“采集链路的设计点在于接口隔离和数据库约束。collector service 不直接关心 RSS/Email 细节，它只依赖 `Collector` 接口。RSS 和 Email 负责把上游内容转换成统一 `CollectedItem`。article service 只负责把这些 item 转成 `Article` 并通过 `OnConflict DoNothing` 和唯一索引区分 inserted 与 duplicated。Redis lock 用来避免同一个 source 并发采集。”

### Follow-up questions

| Question | Answer Outline |
|---|---|
| 如何新增一个采集来源？ | 实现 `Collector` 并注册到 registry |
| 失败如何暴露给前端？ | 返回包含 failed 状态的 collection run |
| 去重由谁保证？ | 数据库唯一索引和 `CreateIfNotExists` |
| RSS 安全风险怎么处理？ | netguard + HTTP timeout + size limit |
| Email 已读如何处理？ | config 中维护 seen message IDs 和 last seen UID |

## 10. Resume Risk Notes

可以说“实现了 RSS/Email 采集编排和文章去重入库”，不要说“采集绝不重复”。

## 11. Completion Checklist

- [ ] 能画出采集调用链。
- [ ] 能解释 run 状态。
- [ ] 能解释 RSS/Email 到 CollectedItem 的转换。
- [ ] 能解释文章去重。
