"use client";

import { useCallback, useEffect, useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import type { DLQItem } from "@/lib/api/types";
import { Badge, Button, EmptyState, ErrorBanner, Panel, SelectInput } from "@/components/ui";

export function DLQPanel() {
  const [items, setItems] = useState<DLQItem[]>([]);
  const [status, setStatus] = useState("pending");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(false);

  const loadItems = useCallback(async () => {
    setError("");
    setLoading(true);
    try {
      const result = await api.listDLQ({ status, limit: 50 });
      setItems(result.items);
    } catch (err: unknown) {
      setError(humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }, [status]);

  useEffect(() => {
    loadItems();
  }, [loadItems]);

  async function replay(item: DLQItem) {
    setError("");
    setNotice("");
    setLoading(true);
    try {
      await api.replayDLQItem(item.id);
      setNotice(`任务 ${item.task_id} 已重新入队`);
      await loadItems();
    } catch (err: unknown) {
      setError(humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  async function markHandled(item: DLQItem) {
    setError("");
    setNotice("");
    setLoading(true);
    try {
      await api.markDLQItemHandled(item.id);
      setNotice(`任务 ${item.task_id} 已标记处理`);
      await loadItems();
    } catch (err: unknown) {
      setError(humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Panel
      title="DLQ 管理"
      actions={
        <div className="flex gap-2">
          <SelectInput className="w-36" value={status} onChange={(event) => setStatus(event.target.value)}>
            <option value="">全部状态</option>
            <option value="pending">待处理</option>
            <option value="replayed">已重放</option>
            <option value="handled">已处理</option>
          </SelectInput>
          <Button type="button" onClick={loadItems} disabled={loading}>
            刷新
          </Button>
        </div>
      }
    >
      <div className="space-y-3">
        <ErrorBanner message={error} />
        {notice ? <div className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{notice}</div> : null}
        {items.length === 0 ? (
          <EmptyState>{loading ? "加载 DLQ 中" : "暂无 DLQ 记录"}</EmptyState>
        ) : (
          <div className="overflow-x-auto rounded-md border border-slate-200">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
                <tr>
                  <th className="px-3 py-2">ID</th>
                  <th className="px-3 py-2">任务</th>
                  <th className="px-3 py-2">Source</th>
                  <th className="px-3 py-2">状态</th>
                  <th className="px-3 py-2">尝试</th>
                  <th className="px-3 py-2">创建</th>
                  <th className="px-3 py-2">错误</th>
                  <th className="px-3 py-2">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 bg-white">
                {items.map((item) => (
                  <tr key={item.id}>
                    <td className="whitespace-nowrap px-3 py-2 font-medium text-slate-900">#{item.id}</td>
                    <td className="max-w-[14rem] truncate px-3 py-2 text-slate-700">{item.task_id}</td>
                    <td className="whitespace-nowrap px-3 py-2">{item.source_id}</td>
                    <td className="whitespace-nowrap px-3 py-2">
                      <Badge tone={statusTone(item.status)}>{item.status}</Badge>
                    </td>
                    <td className="px-3 py-2">{item.attempt}</td>
                    <td className="whitespace-nowrap px-3 py-2 text-slate-500">{formatDate(item.created_at)}</td>
                    <td className="max-w-xs truncate px-3 py-2 text-red-700">{item.error_message}</td>
                    <td className="whitespace-nowrap px-3 py-2">
                      <div className="flex gap-2">
                        <Button type="button" disabled={loading || item.status !== "pending"} onClick={() => replay(item)}>
                          重放
                        </Button>
                        <Button type="button" variant="ghost" disabled={loading || item.status === "handled"} onClick={() => markHandled(item)}>
                          标记处理
                        </Button>
                      </div>
                    </td>
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

function statusTone(status: string) {
  if (status === "pending") {
    return "amber";
  }
  if (status === "replayed") {
    return "blue";
  }
  if (status === "handled") {
    return "green";
  }
  return "slate";
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "short",
    timeStyle: "short"
  }).format(new Date(value));
}
