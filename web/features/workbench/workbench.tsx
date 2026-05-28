"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import { clearSession, readSession } from "@/lib/auth/session";
import type { SessionSnapshot } from "@/lib/auth/session";
import type { Article, CollectionRun, Source } from "@/lib/api/types";
import { AuthPanel } from "@/features/auth/auth-panel";
import { SourceManager } from "@/features/sources/source-manager";
import { ArticleWorkspace } from "@/features/articles/article-workspace";
import { AIPanel } from "@/features/ai/ai-panel";
import { CollectionRunPanel } from "@/features/collection-runs/collection-run-panel";
import { DLQPanel } from "@/features/dlq/dlq-panel";
import { SettingsPanel } from "@/features/settings/settings-panel";
import { Badge, Button, ErrorBanner } from "@/components/ui";

type View = "articles" | "sources" | "runs" | "dlq" | "ai" | "settings";

export function Workbench() {
  const [session, setSession] = useState<SessionSnapshot | null>(() => readSession());
  const [view, setView] = useState<View>("articles");
  const [sources, setSources] = useState<Source[]>([]);
  const [selectedSourceID, setSelectedSourceID] = useState<number | null>(null);
  const [selectedArticle, setSelectedArticle] = useState<Article | null>(null);
  const [latestRun, setLatestRun] = useState<CollectionRun | null>(null);
  const [error, setError] = useState("");

  const reloadSources = useCallback(async () => {
    const result = await api.listSources({ limit: 100 });
    setSources(result.sources);
    setSelectedSourceID((current) => current ?? result.sources[0]?.id ?? null);
  }, []);

  useEffect(() => {
    if (!session) {
      return;
    }
    reloadSources().catch((err: unknown) => setError(humanizeAPIError(err)));
  }, [reloadSources, session]);

  const selectedSource = useMemo(
    () => sources.find((source) => source.id === selectedSourceID) ?? null,
    [selectedSourceID, sources]
  );

  function logout() {
    void api.logout().catch(() => undefined);
    clearSession();
    setSession(null);
    setSources([]);
    setSelectedArticle(null);
    setLatestRun(null);
  }

  if (!session) {
    return <AuthPanel onAuthenticated={setSession} />;
  }

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 border-b border-slate-200 bg-white/92 backdrop-blur">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-3">
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">contentflow</p>
            <h1 className="truncate text-lg font-semibold text-slate-950">内容聚合工作台</h1>
          </div>
          <div className="flex items-center gap-3">
            <Badge tone="blue">{session.user?.email ?? "已登录"}</Badge>
            <Button type="button" variant="ghost" onClick={logout}>
              退出
            </Button>
          </div>
        </div>
      </header>

      <div className="mx-auto grid max-w-7xl grid-cols-1 gap-4 px-4 py-4 lg:grid-cols-[220px_minmax(0,1fr)]">
        <nav className="rounded-lg border border-slate-200 bg-white p-2">
          {[
            ["articles", "文章"] as const,
            ["sources", "来源"] as const,
            ["runs", "采集记录"] as const,
            ["dlq", "DLQ"] as const,
            ["ai", "AI"] as const,
            ["settings", "设置"] as const
          ].map(([id, label]) => (
            <button
              key={id}
              className={`mb-1 flex min-h-10 w-full items-center rounded-md px-3 text-left text-sm font-medium transition ${
                view === id ? "bg-blue-700 text-white" : "text-slate-700 hover:bg-slate-100"
              }`}
              type="button"
              onClick={() => setView(id)}
            >
              {label}
            </button>
          ))}
        </nav>

        <main className="space-y-4">
          <ErrorBanner message={error} />
          {view === "articles" ? (
            <ArticleWorkspace
              sources={sources}
              selectedSourceID={selectedSourceID}
              onSourceChange={setSelectedSourceID}
              selectedArticle={selectedArticle}
              onSelectedArticleChange={setSelectedArticle}
            />
          ) : null}
          {view === "sources" ? (
            <SourceManager
              sources={sources}
              selectedSourceID={selectedSourceID}
              onSelectSource={setSelectedSourceID}
              onSourcesChanged={reloadSources}
              onRunCreated={(run) => {
                setLatestRun(run);
                setView("runs");
              }}
            />
          ) : null}
          {view === "runs" ? (
            <CollectionRunPanel source={selectedSource} latestRun={latestRun} onSelectSource={setSelectedSourceID} sources={sources} />
          ) : null}
          {view === "dlq" ? <DLQPanel /> : null}
          {view === "ai" ? <AIPanel /> : null}
          {view === "settings" ? <SettingsPanel user={session.user} /> : null}
        </main>
      </div>
    </div>
  );
}
