"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import type { CollectionRun, Source, SourcePayload, SourceType, SourceUpdatePayload } from "@/lib/api/types";
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
  const [editURL, setEditURL] = useState("");
  const [editConfig, setEditConfig] = useState("{}");
  const [editConfigDirty, setEditConfigDirty] = useState(false);
  const [editActive, setEditActive] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!selectedSource) {
      setEditName("");
      setEditURL("");
      setEditConfig("{}");
      setEditConfigDirty(false);
      setEditActive(true);
      return;
    }
    setEditName(selectedSource.name);
    setEditURL(selectedSource.url ?? "");
    setEditConfig(JSON.stringify(selectedSource.config ?? {}, null, 2));
    setEditConfigDirty(false);
    setEditActive(selectedSource.is_active);
  }, [selectedSource]);

  async function createSource(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setNotice("");
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
    setNotice("");
    setLoading(true);
    try {
      const payload: SourceUpdatePayload = {
        name: editName.trim() || selectedSource.name,
        url: editURL.trim() || null,
        is_active: editActive
      };
      if (editConfigDirty) {
        payload.config = parseConfig(editConfig);
      }
      await api.updateSource(selectedSource.id, payload);
      await onSourcesChanged();
    } catch (err) {
      setError(err instanceof SyntaxError ? "配置必须是合法 JSON 对象" : humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  async function deleteSelectedSource() {
    if (!selectedSource) {
      return;
    }
    if (!window.confirm(`确认删除来源「${selectedSource.name}」？`)) {
      return;
    }
    setError("");
    setNotice("");
    setLoading(true);
    try {
      await api.deleteSource(selectedSource.id);
      onSelectSource(null);
      await onSourcesChanged();
      setNotice("来源已删除");
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
    setNotice("");
    setLoading(true);
    try {
      const result = await api.collectSource(selectedSource.id);
      await onSourcesChanged();
      if (result.collection_run) {
        onRunCreated(result.collection_run);
      }
      if (result.collection_task) {
        setNotice(`采集任务已入队：${result.collection_task.task_id}`);
      }
    } catch (err) {
      setError(humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  function selectSource(source: Source) {
    onSelectSource(source.id);
    setNotice("");
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <Panel title="来源管理">
        <ErrorBanner message={error} />
        {notice ? <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{notice}</div> : null}
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
              <label className="block space-y-1.5 text-sm font-medium text-slate-700">
                <span>URL</span>
                <TextInput value={editURL} onChange={(event) => setEditURL(event.target.value)} />
              </label>
              <label className="block space-y-1.5 text-sm font-medium text-slate-700">
                <span>配置 JSON</span>
                <textarea
                  className="min-h-28 w-full rounded-md border border-slate-300 bg-white px-3 py-2 font-mono text-sm outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
                  value={editConfig}
                  onChange={(event) => {
                    setEditConfig(event.target.value);
                    setEditConfigDirty(true);
                  }}
                />
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
              <Button className="w-full" type="button" variant="danger" onClick={deleteSelectedSource} disabled={loading}>
                删除来源
              </Button>
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
