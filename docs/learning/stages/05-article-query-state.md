# Stage 5: Article 查询、搜索和状态

## 0. 术语预解释

- 查询：从数据库读取符合条件的数据。
- 全文搜索：根据文本内容查找匹配文档，而不是只按精确字段过滤。
- JOIN：数据库中把多张表按关系组合查询。
- Upsert：不存在就插入，存在就更新。
- DTO：对外返回或跨层传递的数据结构。

## 1. Learning Target

你需要能解释文章列表、详情、搜索、已读和收藏状态如何按用户隔离。

## 2. Files to Read

- `internal/module/article/route.go`
- `internal/module/article/handler.go`
- `internal/module/article/service.go`
- `internal/module/article/repository.go`
- `internal/module/article/model.go`
- `internal/module/article/cache.go`
- `migrations/000001_create_initial_tables.up.sql`
- `migrations/000004_create_article_search_index.up.sql`

## 3. Reading Sequence

1. 先看 `articles` 和 `article_states` 表。
2. 看 article route。
3. 看 handler 的 query 参数解析。
4. 看 service 的 cache 和 DTO 处理。
5. 看 repository 的 JOIN 查询。
6. 看全文搜索索引 migration。

## 4. Key Code Objects

| Name | Type | File | Why It Matters |
|---|---|---|---|
| `Article` | struct | `article/model.go` | 文章主表映射 |
| `ArticleState` | struct | `article/model.go` | 用户维度阅读/收藏状态 |
| `ArticleWithState` | struct | `article/repository.go` | 文章和状态合并后的查询结果 |
| `ListByUser` | method | `article/repository.go` | 用户隔离查询核心 |
| `applyArticleFilters` | function | `article/repository.go` | source、search、read、saved 过滤 |
| `UpsertState` | method | `article/repository.go` | 更新 read/save 状态 |

## 5. Hands-On Checks

```fish
go test ./internal/module/article
```

## 6. Source Notes

- 列表查询 JOIN `sources`，确保 source 属于当前 user。
- 列表查询 LEFT JOIN `article_states`，没有状态时默认 unread/unsaved。
- 列表 DTO 会清空 `Content`，详情才返回正文。
- 全文搜索使用 PostgreSQL `to_tsvector` 和 `plainto_tsquery`。
- 状态更新先确认文章属于用户，再 upsert `article_states`。
- 更新状态后删除用户 article list cache。

## 7. Diagram Task

```text
GET /articles?q=...
  -> Handler.List
  -> Service.ListArticles
  -> cache.GetList
  -> Repository.ListByUser
  -> JOIN sources + LEFT JOIN article_states
  -> cache.SetList
```

## 8. Self-Test

1. 为什么文章查询要 JOIN sources？
2. `includeContent=false` 时 repository 选什么 content？
3. `is_read` 为空和 false 有什么区别？
4. 全文搜索查询语句在哪里？
5. 状态更新为什么先调用 `FindByUserAndID`？
6. `read_at` 什么时候设为 nil？
7. article list cache key 包含哪些字段？
8. 文章列表为什么不返回 content？

## 9. Interview Drill

### 30-second explanation

“文章模块通过 JOIN sources 保证用户只能看到自己的来源下的文章，再 LEFT JOIN article_states 获取已读和收藏状态。列表不返回正文以减少响应体，详情才返回 content。状态更新用 upsert 维护每个 user + article 的唯一状态行。”

### 2-minute explanation

“article repository 是这个模块的重点。文章本身属于 source，而 source 属于 user，所以查询时不能只按 articleID 查，必须 JOIN sources 并过滤 userID。`ArticleWithState` 把文章字段和状态字段合并。全文搜索在 PostgreSQL 里用 tsvector 索引支持，状态更新会先确认权限，再用 upsert 写 `article_states`，最后删除该用户的 article list cache。”

### Follow-up questions

| Question | Answer Outline |
|---|---|
| 为什么 article 表没有 user_id？ | user 关系通过 source 间接确定 |
| 如何查收藏文章？ | `is_saved` query -> `applyArticleFilters` |
| 为什么列表不返回正文？ | 降低响应体和缓存体积 |
| 搜索有什么局限？ | simple text search，没有排名优化或多语言分词优化 |
| 状态如何保证唯一？ | `UNIQUE(user_id, article_id)` |

## 10. Resume Risk Notes

可以说“实现了用户隔离的文章查询、全文搜索和阅读/收藏状态”，不要说“搜索效果达到生产搜索引擎级别”。

## 11. Completion Checklist

- [ ] 能解释 article 和 source 的关系。
- [ ] 能解释 `ArticleWithState`。
- [ ] 能解释搜索过滤。
- [ ] 能解释 upsert 状态。
