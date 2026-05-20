"use client";

import { useCallback, useEffect, useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import type { CollectionRun, Source } from "@/lib/api/types";
import { Badge, Button, EmptyState, ErrorBanner, Panel, SelectInput } from "@/components/ui";

type CollectionRunPanelProps = {
  source: Source | null;
  sources: Source[];
  latestRun: CollectionRun | null;
  onSelectSource: (sourceID: number | null) => void;
};

export function CollectionRunPanel({ source, sources, latestRun, onSelectSource }: CollectionRunPanelProps) {
  const [runs, setRuns] = useState<CollectionRun[]>([]);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const loadRuns = useCallback(async () => {
    if (!source) {
      return;
    }
    setError("");
    setLoading(true);
    try {
      const result = await api.listCollectionRuns(source.id, { status, limit: 30 });
      setRuns(result.collection_runs);
    } finally {
      setLoading(false);
    }
  }, [source, status]);

  useEffect(() => {
    if (source) {
      loadRuns().catch((err: unknown) => setError(humanizeAPIError(err)));
    }
  }, [loadRuns, source]);

  const visibleRuns = !source
    ? []
    : latestRun && latestRun.source_id === source.id && !runs.some((run) => run.run_id === latestRun.run_id)
      ? [latestRun, ...runs]
      : runs;

  return (
    <Panel
      title="采集记录"
      actions={
        <div className="flex gap-2">
          <SelectInput
            className="w-44"
            value={source?.id ?? ""}
            onChange={(event) => onSelectSource(event.target.value ? Number(event.target.value) : null)}
          >
            <option value="">选择来源</option>
            {sources.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </SelectInput>
          <Button type="button" onClick={loadRuns} disabled={!source || loading}>
            刷新
          </Button>
        </div>
      }
    >
      <div className="space-y-3">
        <ErrorBanner message={error} />
        <SelectInput className="w-40" value={status} onChange={(event) => setStatus(event.target.value)}>
          <option value="">全部状态</option>
          <option value="running">运行中</option>
          <option value="success">成功</option>
          <option value="failed">失败</option>
        </SelectInput>
        {!source ? (
          <EmptyState>选择来源后查看采集历史。</EmptyState>
        ) : visibleRuns.length === 0 ? (
          <EmptyState>{loading ? "加载采集记录中" : "暂无采集记录"}</EmptyState>
        ) : (
          <div className="overflow-x-auto rounded-md border border-slate-200">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                <tr>
                  <th className="px-3 py-2">Run</th>
                  <th className="px-3 py-2">状态</th>
                  <th className="px-3 py-2">抓取</th>
                  <th className="px-3 py-2">新增</th>
                  <th className="px-3 py-2">重复</th>
                  <th className="px-3 py-2">开始</th>
                  <th className="px-3 py-2">错误</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 bg-white">
                {visibleRuns.map((run) => (
                  <tr key={run.run_id}>
                    <td className="whitespace-nowrap px-3 py-2 font-medium text-slate-900">#{run.run_id}</td>
                    <td className="whitespace-nowrap px-3 py-2">
                      <Badge tone={runTone(run.status)}>{run.status}</Badge>
                    </td>
                    <td className="px-3 py-2">{run.fetched_count}</td>
                    <td className="px-3 py-2">{run.inserted_count}</td>
                    <td className="px-3 py-2">{run.duplicated_count}</td>
                    <td className="whitespace-nowrap px-3 py-2 text-slate-500">{run.started_at ? formatDate(run.started_at) : "-"}</td>
                    <td className="max-w-xs truncate px-3 py-2 text-red-700">{run.error_message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Panel>
  );
}

function runTone(status: string) {
  if (status === "success") {
    return "green";
  }
  if (status === "failed") {
    return "red";
  }
  if (status === "running") {
    return "blue";
  }
  return "slate";
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "short",
    timeStyle: "short"
  }).format(new Date(value));
}
