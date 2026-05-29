"use client";

import { FormEvent, useEffect, useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import type { AISettings, AISettingsPayload, AuthUser } from "@/lib/api/types";
import { Badge, Button, ErrorBanner, Panel, SelectInput, TextInput } from "@/components/ui";

type SettingsPanelProps = {
  user: AuthUser | null;
};

export function SettingsPanel({ user }: SettingsPanelProps) {
  const [settings, setSettings] = useState<AISettings | null>(null);
  const [form, setForm] = useState({
    provider: "local",
    baseURL: "https://api.openai.com/v1",
    model: "",
    embeddingModel: "text-embedding-3-small",
    apiKey: ""
  });
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let active = true;
    setError("");
    api
      .getAISettings()
      .then((result) => {
        if (!active) {
          return;
        }
        applySettings(result.settings);
      })
      .catch((err: unknown) => {
        if (active) {
          setError(humanizeAPIError(err));
        }
      });
    return () => {
      active = false;
    };
  }, []);

  function applySettings(next: AISettings) {
    setSettings(next);
    setForm({
      provider: next.provider || "local",
      baseURL: next.base_url || "https://api.openai.com/v1",
      model: next.model || "",
      embeddingModel: next.embedding_model || "text-embedding-3-small",
      apiKey: ""
    });
  }

  async function saveSettings(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await updateSettings(false);
  }

  async function clearAPIKey() {
    await updateSettings(true);
  }

  async function updateSettings(clearKey: boolean) {
    setError("");
    setNotice("");
    if (form.provider === "openai-compatible" && !form.model.trim()) {
      setError("请填写 OpenAI-compatible 的 Chat model");
      return;
    }
    setLoading(true);
    try {
      const payload: AISettingsPayload = {
        provider: form.provider,
        base_url: form.baseURL.trim(),
        model: form.model.trim(),
        embedding_model: form.embeddingModel.trim()
      };
      if (clearKey) {
        payload.api_key = "";
      } else if (form.apiKey.trim()) {
        payload.api_key = form.apiKey.trim();
      }
      const result = await api.updateAISettings(payload);
      applySettings(result.settings);
      setNotice(clearKey ? "AI API key 已清除" : "AI 设置已保存");
    } catch (err: unknown) {
      setError(humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
      <Panel title="账号">
        <dl className="grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-slate-500">邮箱</dt>
            <dd className="font-medium text-slate-900">{user?.email ?? "已登录"}</dd>
          </div>
          <div>
            <dt className="text-slate-500">显示名</dt>
            <dd className="font-medium text-slate-900">{user?.display_name || user?.email || "已登录"}</dd>
          </div>
          <div>
            <dt className="text-slate-500">AI key</dt>
            <dd className="mt-1">
              <Badge tone={settings?.has_api_key ? "green" : "slate"}>{settings?.has_api_key ? "已保存" : "未设置"}</Badge>
            </dd>
          </div>
        </dl>
      </Panel>

      <Panel title="AI 设置">
        <form className="space-y-3" onSubmit={saveSettings}>
          <ErrorBanner message={error} />
          {notice ? <div className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{notice}</div> : null}

          <label className="block space-y-1.5 text-sm font-medium text-slate-700">
            <span>Provider</span>
            <SelectInput value={form.provider} onChange={(event) => setForm({ ...form, provider: event.target.value })}>
              <option value="local">Local</option>
              <option value="openai-compatible">OpenAI-compatible</option>
            </SelectInput>
          </label>

          <label className="block space-y-1.5 text-sm font-medium text-slate-700">
            <span>Base URL</span>
            <TextInput value={form.baseURL} onChange={(event) => setForm({ ...form, baseURL: event.target.value })} />
          </label>

          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block space-y-1.5 text-sm font-medium text-slate-700">
              <span>Chat model</span>
              <TextInput value={form.model} onChange={(event) => setForm({ ...form, model: event.target.value })} />
            </label>
            <label className="block space-y-1.5 text-sm font-medium text-slate-700">
              <span>Embedding model</span>
              <TextInput value={form.embeddingModel} onChange={(event) => setForm({ ...form, embeddingModel: event.target.value })} />
            </label>
          </div>

          <label className="block space-y-1.5 text-sm font-medium text-slate-700">
            <span>API key</span>
            <TextInput
              autoComplete="off"
              placeholder={settings?.has_api_key ? "留空则保留已保存 key" : "输入 API key"}
              type="password"
              value={form.apiKey}
              onChange={(event) => setForm({ ...form, apiKey: event.target.value })}
            />
          </label>

          <div className="flex flex-wrap gap-2">
            <Button type="submit" variant="primary" disabled={loading}>
              保存
            </Button>
            <Button type="button" variant="secondary" disabled={loading || !settings?.has_api_key} onClick={clearAPIKey}>
              清除密钥
            </Button>
          </div>
        </form>
      </Panel>
    </div>
  );
}
