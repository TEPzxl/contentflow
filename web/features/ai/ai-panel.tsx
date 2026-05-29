"use client";

import { useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import type { DailyDigest, RAGAnswer } from "@/lib/api/types";
import { Badge, Button, EmptyState, ErrorBanner, Panel, TextInput } from "@/components/ui";

export function AIPanel() {
  const [date, setDate] = useState(() => new Date().toISOString().slice(0, 10));
  const [digest, setDigest] = useState<DailyDigest | null>(null);
  const [query, setQuery] = useState("");
  const [answer, setAnswer] = useState<RAGAnswer | null>(null);
  const [digestError, setDigestError] = useState("");
  const [ragError, setRAGError] = useState("");
  const [digestLoading, setDigestLoading] = useState(false);
  const [ragLoading, setRAGLoading] = useState(false);

  async function generateDigest() {
    setDigestError("");
    setDigestLoading(true);
    try {
      const result = await api.generateDigest(date);
      setDigest(result.digest);
    } catch (err: unknown) {
      setDigestError(humanizeAPIError(err));
    } finally {
      setDigestLoading(false);
    }
  }

  async function loadDigest() {
    setDigestError("");
    setDigestLoading(true);
    try {
      const result = await api.getDigest(date);
      setDigest(result.digest);
    } catch (err: unknown) {
      setDigestError(humanizeAPIError(err));
    } finally {
      setDigestLoading(false);
    }
  }

  async function ask() {
    setRAGError("");
    setRAGLoading(true);
    try {
      const result = await api.ragSearch({ query, limit: 5 });
      setAnswer(result.answer);
    } catch (err: unknown) {
      setRAGError(humanizeAPIError(err));
    } finally {
      setRAGLoading(false);
    }
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
      <Panel
        title="Daily Digest"
        actions={
          <div className="flex gap-2">
            <Button type="button" disabled={digestLoading} onClick={loadDigest}>
              读取
            </Button>
            <Button type="button" disabled={digestLoading} onClick={generateDigest}>
              生成
            </Button>
          </div>
        }
      >
        <div className="space-y-3">
          <ErrorBanner message={digestError} />
          <TextInput type="date" value={date} onChange={(event) => setDate(event.target.value)} />
          {digest ? (
            <div className="rounded-md border border-slate-200 bg-slate-50 p-4">
              <div className="mb-3 flex flex-wrap gap-2">
                <Badge tone="green">{digest.status}</Badge>
                <Badge tone="slate">{digest.model}</Badge>
                <Badge tone="slate">{digest.prompt_version}</Badge>
              </div>
              <p className="whitespace-pre-wrap text-sm leading-6 text-slate-700">{digest.summary}</p>
              <p className="mt-3 text-xs text-slate-500">引用文章数：{digest.article_ids.length}</p>
            </div>
          ) : (
            <EmptyState>选择日期后读取或生成日报。</EmptyState>
          )}
        </div>
      </Panel>

      <Panel
        title="RAG 搜索"
        actions={
          <Button type="button" disabled={ragLoading || !query.trim()} onClick={ask}>
            提问
          </Button>
        }
      >
        <div className="space-y-3">
          <ErrorBanner message={ragError} />
          <TextInput placeholder="输入问题，例如：Kafka 重试失败如何处理" value={query} onChange={(event) => setQuery(event.target.value)} />
          {answer ? (
            <div className="space-y-4">
              <div className="rounded-md border border-slate-200 bg-white p-4">
                <div className="mb-3 flex flex-wrap gap-2">
                  <Badge tone="slate">{answer.model}</Badge>
                  <Badge tone="slate">{answer.prompt_version}</Badge>
                </div>
                <p className="whitespace-pre-wrap text-sm leading-6 text-slate-700">{answer.answer}</p>
              </div>
              <div>
                <h3 className="text-sm font-semibold text-slate-950">引用</h3>
                <ul className="mt-2 divide-y divide-slate-200 rounded-md border border-slate-200">
                  {answer.citations.map((item) => (
                    <li key={item.article_id} className="p-3 text-sm">
                      <div className="font-medium text-slate-900">{item.title}</div>
                      <p className="mt-1 line-clamp-2 text-slate-500">{item.snippet}</p>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          ) : (
            <EmptyState>RAG 回答会返回引用文章，便于追溯来源。</EmptyState>
          )}
        </div>
      </Panel>
    </div>
  );
}
