"use client";

import { FormEvent, useMemo, useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import type { CollectionRun, Source, SourcePayload, SourceType } from "@/lib/api/types";
import { Badge, Button, EmptyState, ErrorBanner, Panel, SelectInput, TextInput } from "@/components/ui";

type SourceManagerProps = {
  sources: Source[];
  selectedSourceID: number | null;
  onSelectSource: (sourceID: number | null) => void;
  onSourcesChanged: () => Promise<void>;
  onRunCreated: (run: CollectionRun) => void;
};

export function SourceManager({
  sources,
  selectedSourceID,
  onSelectSource,
  onSourcesChanged,
  onRunCreated
}: SourceManagerProps) {
  const selectedSource = useMemo(
    () => sources.find((source) => source.id === selectedSourceID) ?? null,
    [selectedSourceID, sources]
  );
  const [form, setForm] = useState({
    name: "",
    type: "rss" as SourceType,
    url: "",
    config: "{}"
  });
  const [editName, setEditName] = useState("");
  const [editActive, setEditActive] = useState(true);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function createSource(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      const payload: SourcePayload = {
        name: form.name.trim(),
        type: form.type,
        url: form.url.trim() || null,
        config: parseConfig(form.config)
      };
      const result = await api.createSource(payload);
      await onSourcesChanged();
      onSelectSource(result.source.id);
      setForm({ name: "", type: "rss", url: "", config: "{}" });
    } catch (err) {
      setError(err instanceof SyntaxError ? "配置必须是合法 JSON" : humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  async function updateSelectedSource() {
    if (!selectedSource) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      await api.updateSource(selectedSource.id, {
        name: editName || selectedSource.name,
        is_active: editActive
      });
      await onSourcesChanged();
    } catch (err) {
      setError(humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  async function collectSelectedSource() {
    if (!selectedSource) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      const result = await api.collectSource(selectedSource.id);
      await onSourcesChanged();
      onRunCreated(result.collection_run);
    } catch (err) {
      setError(humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  function selectSource(source: Source) {
    onSelectSource(source.id);
    setEditName(source.name);
    setEditActive(source.is_active);
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <Panel title="来源管理">
        <ErrorBanner message={error} />
        <div className="mt-4 overflow-hidden rounded-md border border-slate-200">
          {sources.length === 0 ? (
            <EmptyState>还没有来源，先创建 RSS 或 email source。</EmptyState>
          ) : (
            <ul className="divide-y divide-slate-200">
              {sources.map((source) => (
                <li key={source.id}>
                  <button
                    className={`grid w-full gap-2 px-4 py-3 text-left transition hover:bg-slate-50 ${
                      selectedSourceID === source.id ? "bg-blue-50" : ""
                    }`}
                    type="button"
                    onClick={() => selectSource(source)}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="truncate text-sm font-medium text-slate-950">{source.name}</span>
                      <div className="flex items-center gap-2">
                        <Badge tone={source.type === "rss" ? "blue" : "amber"}>{source.type}</Badge>
                        <Badge tone={source.is_active ? "green" : "slate"}>{source.is_active ? "启用" : "停用"}</Badge>
                      </div>
                    </div>
                    <span className="truncate text-xs text-slate-500">{source.url ?? "无 URL 配置"}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Panel>

      <div className="space-y-4">
        <Panel title="创建来源">
          <form className="space-y-3" onSubmit={createSource}>
            <label className="block space-y-1.5 text-sm font-medium text-slate-700">
              <span>名称</span>
              <TextInput required value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
            </label>
            <label className="block space-y-1.5 text-sm font-medium text-slate-700">
              <span>类型</span>
              <SelectInput value={form.type} onChange={(event) => setForm({ ...form, type: event.target.value as SourceType })}>
                <option value="rss">RSS</option>
                <option value="email">Email</option>
              </SelectInput>
            </label>
            <label className="block space-y-1.5 text-sm font-medium text-slate-700">
              <span>URL</span>
              <TextInput value={form.url} onChange={(event) => setForm({ ...form, url: event.target.value })} />
            </label>
            <label className="block space-y-1.5 text-sm font-medium text-slate-700">
              <span>配置 JSON</span>
              <textarea
                className="min-h-28 w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-mono outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
                value={form.config}
                onChange={(event) => setForm({ ...form, config: event.target.value })}
              />
            </label>
            <Button className="w-full" type="submit" variant="primary" disabled={loading}>
              创建
            </Button>
          </form>
        </Panel>

        <Panel title="当前来源">
          {selectedSource ? (
            <div className="space-y-3">
              <label className="block space-y-1.5 text-sm font-medium text-slate-700">
                <span>名称</span>
                <TextInput value={editName || selectedSource.name} onChange={(event) => setEditName(event.target.value)} />
              </label>
              <label className="flex items-center gap-2 text-sm text-slate-700">
                <input
                  checked={editActive}
                  className="h-4 w-4 rounded border-slate-300"
                  type="checkbox"
                  onChange={(event) => setEditActive(event.target.checked)}
                />
                启用自动采集
              </label>
              <div className="grid grid-cols-2 gap-2">
                <Button type="button" onClick={updateSelectedSource} disabled={loading}>
                  保存
                </Button>
                <Button type="button" variant="primary" onClick={collectSelectedSource} disabled={loading}>
                  手动采集
                </Button>
              </div>
            </div>
          ) : (
            <EmptyState>选择一个来源后可以编辑和触发采集。</EmptyState>
          )}
        </Panel>
      </div>
    </div>
  );
}

function parseConfig(raw: string): Record<string, unknown> {
  const value = JSON.parse(raw || "{}") as unknown;
  if (value === null || Array.isArray(value) || typeof value !== "object") {
    throw new SyntaxError("config must be object");
  }
  return value as Record<string, unknown>;
}
