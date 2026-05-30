# Stage 7: AI 模块和可替换 Assistant

## 0. 术语预解释

- AI provider：提供摘要、向量或问答能力的实现，可以是本地算法，也可以是外部模型服务。
- Assistant：项目中抽象 AI provider 的接口。
- Embedding：把文本转换成数字向量，方便计算相似度。
- Cosine similarity：一种衡量两个向量方向相似程度的算法。
- RAG：先检索相关内容，再基于内容生成答案的模式。
- Citation：答案引用的来源文章信息。

## 1. Learning Target

你需要能准确解释 AI 模块已实现什么、没有实现什么，以及为什么默认不是外部 LLM。

## 2. Files to Read

- `internal/module/ai/handler.go`
- `internal/module/ai/service.go`
- `internal/module/ai/assistant.go`
- `internal/module/ai/repository.go`
- `internal/module/ai/worker.go`
- `internal/module/ai/model.go`
- `migrations/000003_create_ai_feature_tables.up.sql`
- `migrations/000005_add_embedding_lookup_index.up.sql`

## 3. Reading Sequence

1. route 和 handler。
2. `Assistant` interface。
3. `ExtractiveAssistant` 默认实现。
4. summary request 和 worker。
5. embedding 和 similar articles。
6. digest 和 RAG search。
7. repository 和 migration。

## 4. Key Code Objects

| Name | Type | File | Why It Matters |
|---|---|---|---|
| `Assistant` | interface | `assistant.go` | AI provider 扩展点 |
| `ExtractiveAssistant` | struct | `assistant.go` | 默认本地实现 |
| `Service.RequestSummary` | method | `service.go` | 摘要任务入口 |
| `ProcessNextSummary` | method | `service.go` | summary worker 执行入口 |
| `GenerateEmbedding` | method | `service.go` | 生成文章向量 |
| `SimilarArticles` | method | `service.go` | 计算相似文章 |
| `RAGSearch` | method | `service.go` | RAG 风格搜索入口 |

## 5. Hands-On Checks

```fish
go test ./internal/module/ai
```

## 6. Source Notes

- 默认 Assistant 不需要 API key。
- Summary 请求先写 `article_summaries` pending 记录。
- `SummaryWorker` 循环调用 `ProcessNextSummary`。
- `ClaimNextSummary` 用 `FOR UPDATE SKIP LOCKED` 风格锁定任务。
- Embedding 存在 `article_embeddings.embedding_json`。
- SimilarArticles 从同用户 embedding 中取最多 500 条计算 cosine similarity。
- RAGSearch 先用 article query 搜索，再用 Assistant 生成 answer 和 citations。

## 7. Diagram Task

```text
POST /articles/:id/summary
  -> Handler.RequestSummary
  -> Service.RequestSummary
  -> Repository.EnqueueSummary
  -> SummaryWorker.Run
  -> ProcessNextSummary
  -> Assistant.Summarize
  -> Repository.CompleteSummary
```

## 8. Self-Test

1. 默认 Assistant 的 model 名是什么？
2. RequestSummary 什么时候直接返回 cached summary？
3. summary job 失败如何设置下一次尝试时间？
4. embedding vector 如何生成？
5. SimilarArticles 如何过滤当前文章？
6. RAGSearch 为什么依赖 article full text search？
7. Digest 默认取多少篇文章？
8. 为什么不能说“接入了真实 LLM”？
9. 如果以后接 OpenAI，需要改哪个抽象？
10. AI 表有哪些唯一约束？

## 9. Interview Drill

### 30-second explanation

“AI 模块用 Assistant 接口隔离 provider。默认实现是本地 extractive/hash 算法，不依赖外部 LLM。摘要是异步任务，embedding 存 PostgreSQL JSONB，相似文章用 cosine similarity，RAG 搜索基于文章全文搜索和本地 citations 生成。”

### 2-minute explanation

“这个模块的重点不是大模型本身，而是扩展边界。`Assistant` 有 Summarize、Embed、Digest、Answer 四个方法。service 负责鉴权后的文章范围、任务入队、失败重试、embedding upsert、相似度计算和 RAG 输出。默认 `ExtractiveAssistant` 让项目不需要密钥也能测试端到端流程；如果换真实 provider，应该实现同一个接口，而不是改 handler 或 repository。”

### Follow-up questions

| Question | Answer Outline |
|---|---|
| 为什么默认本地算法？ | 可测试、无密钥、稳定 |
| 如何替换外部模型？ | 新实现 `Assistant` 接口并在 app 装配替换 |
| RAG 有什么局限？ | 不是向量数据库检索，不是 LLM 生成 |
| Summary 为什么异步？ | 生成可能慢，适合后台 worker |
| Embedding 存 JSONB 有什么取舍？ | 简单可实现，但不适合大规模向量检索 |

## 10. Resume Risk Notes

可以说“实现了可替换 Assistant 抽象和本地 AI 功能链路”，不要说“接入外部大模型”。

## 11. Completion Checklist

- [ ] 能解释 Assistant 接口。
- [ ] 能解释 summary worker。
- [ ] 能解释 embedding 和 similarity。
- [ ] 能解释 RAG 的真实边界。
