"use client";

import { useCallback, useEffect, useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import type { Article, Source } from "@/lib/api/types";
import { Badge, Button, EmptyState, ErrorBanner, Panel, SelectInput, TextInput } from "@/components/ui";

type ArticleWorkspaceProps = {
  sources: Source[];
  selectedSourceID: number | null;
  onSourceChange: (sourceID: number | null) => void;
  selectedArticle: Article | null;
  onSelectedArticleChange: (article: Article | null) => void;
};

export function ArticleWorkspace({
  sources,
  selectedSourceID,
  onSourceChange,
  selectedArticle,
  onSelectedArticleChange
}: ArticleWorkspaceProps) {
  const [articles, setArticles] = useState<Article[]>([]);
  const [query, setQuery] = useState("");
  const [readFilter, setReadFilter] = useState("");
  const [savedOnly, setSavedOnly] = useState(false);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const loadArticles = useCallback(async (nextOffset = offset) => {
    setError("");
    setLoading(true);
    try {
      const result = await api.listArticles({
        source_id: selectedSourceID ?? undefined,
        q: query,
        is_read: readFilter === "" ? undefined : readFilter === "read",
        is_saved: savedOnly || undefined,
        limit: 20,
        offset: nextOffset
      });
      setArticles(result.articles);
      setTotal(result.total);
      setOffset(result.offset);
      if (result.articles.length > 0 && !selectedArticle) {
        onSelectedArticleChange(result.articles[0]);
      }
    } finally {
      setLoading(false);
    }
  }, [offset, onSelectedArticleChange, query, readFilter, savedOnly, selectedArticle, selectedSourceID]);

  useEffect(() => {
    loadArticles(0).catch((err: unknown) => setError(humanizeAPIError(err)));
  }, [loadArticles]);

  async function updateRead(article: Article, isRead: boolean) {
    const result = await api.markArticleRead(article.id, isRead);
    onSelectedArticleChange(result.article);
    await loadArticles(offset);
  }

  async function updateSaved(article: Article, isSaved: boolean) {
    const result = await api.saveArticle(article.id, isSaved);
    onSelectedArticleChange(result.article);
    await loadArticles(offset);
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(360px,430px)_minmax(0,1fr)]">
      <Panel title="文章列表">
        <div className="space-y-3">
          <ErrorBanner message={error} />
          <div className="grid gap-2 sm:grid-cols-2">
            <TextInput placeholder="搜索标题或内容" value={query} onChange={(event) => setQuery(event.target.value)} />
            <SelectInput
              value={selectedSourceID ?? ""}
              onChange={(event) => onSourceChange(event.target.value ? Number(event.target.value) : null)}
            >
              <option value="">全部来源</option>
              {sources.map((source) => (
                <option key={source.id} value={source.id}>
                  {source.name}
                </option>
              ))}
            </SelectInput>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <SelectInput className="w-36" value={readFilter} onChange={(event) => setReadFilter(event.target.value)}>
              <option value="">全部状态</option>
              <option value="unread">未读</option>
              <option value="read">已读</option>
            </SelectInput>
            <label className="flex min-h-9 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm text-slate-700">
              <input checked={savedOnly} type="checkbox" onChange={(event) => setSavedOnly(event.target.checked)} />
              只看收藏
            </label>
            <Button type="button" onClick={() => loadArticles(0)} disabled={loading}>
              搜索
            </Button>
          </div>
          {articles.length === 0 ? (
            <EmptyState>{loading ? "加载文章中" : "没有匹配文章"}</EmptyState>
          ) : (
            <ul className="divide-y divide-slate-200 overflow-hidden rounded-md border border-slate-200">
              {articles.map((article) => (
                <li key={article.id}>
                  <button
                    className={`block w-full px-4 py-3 text-left hover:bg-slate-50 ${
                      selectedArticle?.id === article.id ? "bg-blue-50" : ""
                    }`}
                    type="button"
                    onClick={() => onSelectedArticleChange(article)}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="line-clamp-1 text-sm font-medium text-slate-950">{article.title}</span>
                      <div className="flex shrink-0 gap-1">
                        {article.is_saved ? <Badge tone="amber">收藏</Badge> : null}
                        <Badge tone={article.is_read ? "slate" : "green"}>{article.is_read ? "已读" : "未读"}</Badge>
                      </div>
                    </div>
                    <p className="mt-1 line-clamp-2 text-xs leading-5 text-slate-500">{article.summary || article.content}</p>
                  </button>
                </li>
              ))}
            </ul>
          )}
          <div className="flex items-center justify-between text-sm text-slate-500">
            <span>
              {offset + 1}-{Math.min(offset + articles.length, total)} / {total}
            </span>
            <div className="flex gap-2">
              <Button type="button" disabled={offset === 0 || loading} onClick={() => loadArticles(Math.max(0, offset - 20))}>
                上一页
              </Button>
              <Button type="button" disabled={offset + articles.length >= total || loading} onClick={() => loadArticles(offset + 20)}>
                下一页
              </Button>
            </div>
          </div>
        </div>
      </Panel>

      <Panel
        title="文章详情"
        actions={
          selectedArticle ? (
            <div className="flex gap-2">
              <Button type="button" onClick={() => updateRead(selectedArticle, !selectedArticle.is_read)}>
                {selectedArticle.is_read ? "标为未读" : "标为已读"}
              </Button>
              <Button type="button" onClick={() => updateSaved(selectedArticle, !selectedArticle.is_saved)}>
                {selectedArticle.is_saved ? "取消收藏" : "收藏"}
              </Button>
            </div>
          ) : null
        }
      >
        {selectedArticle ? (
          <article className="prose prose-slate max-w-none">
            <div className="mb-4 flex flex-wrap items-center gap-2">
              <Badge tone="blue">{selectedArticle.source_type}</Badge>
              {selectedArticle.published_at ? <span className="text-xs text-slate-500">{formatDate(selectedArticle.published_at)}</span> : null}
            </div>
            <h2 className="text-2xl font-semibold text-slate-950">{selectedArticle.title}</h2>
            {selectedArticle.author ? <p className="text-sm text-slate-500">{selectedArticle.author}</p> : null}
            <p className="whitespace-pre-wrap text-sm leading-7 text-slate-700">{selectedArticle.content || selectedArticle.summary}</p>
            {selectedArticle.url ? (
              <a className="text-sm font-medium text-blue-700 hover:text-blue-900" href={selectedArticle.url} rel="noreferrer" target="_blank">
                打开原文
              </a>
            ) : null}
          </article>
        ) : (
          <EmptyState>选择一篇文章查看正文。</EmptyState>
        )}
      </Panel>
    </div>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(new Date(value));
}
