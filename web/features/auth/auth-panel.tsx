"use client";

import { FormEvent, useEffect, useState } from "react";
import { api, humanizeAPIError } from "@/lib/api/client";
import { saveSession } from "@/lib/auth/session";
import type { SessionSnapshot } from "@/lib/auth/session";
import { Button, ErrorBanner, TextInput } from "@/components/ui";

type AuthMode = "login" | "register";

export function AuthPanel({
  initialMode = "login",
  onAuthenticated
}: {
  initialMode?: AuthMode;
  onAuthenticated: (session: SessionSnapshot) => void;
}) {
  const [mode, setMode] = useState<AuthMode>(initialMode);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  function resetForMode(nextMode: AuthMode) {
    setMode(nextMode);
    setEmail("");
    setPassword("");
    setDisplayName("");
    setError("");
  }

  useEffect(() => {
    resetForMode(initialMode);
  }, [initialMode]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setLoading(true);

    try {
      if (mode === "register") {
        await api.register({ email, password, display_name: displayName });
      }
      const tokens = await api.login({ email, password });
      onAuthenticated(saveSession(tokens));
    } catch (err) {
      setError(humanizeAPIError(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="grid min-h-screen place-items-center px-4 py-8">
      <section className="w-full max-w-md rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="mb-6">
          <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">contentflow</p>
          <h1 className="mt-2 text-2xl font-semibold text-slate-950">
            {mode === "login" ? "登录工作台" : "创建账号"}
          </h1>
        </div>

        <form className="space-y-4" onSubmit={submit}>
          <ErrorBanner message={error} />
          {mode === "register" ? (
            <label className="block space-y-1.5 text-sm font-medium text-slate-700">
              <span>显示名称</span>
              <TextInput value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
            </label>
          ) : null}
          <label className="block space-y-1.5 text-sm font-medium text-slate-700">
            <span>邮箱</span>
            <TextInput type="email" value={email} required onChange={(event) => setEmail(event.target.value)} />
          </label>
          <label className="block space-y-1.5 text-sm font-medium text-slate-700">
            <span>密码</span>
            <TextInput
              type="password"
              value={password}
              required
              minLength={8}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
          <Button className="w-full" type="submit" variant="primary" disabled={loading}>
            {loading ? "处理中" : mode === "login" ? "登录" : "注册并登录"}
          </Button>
        </form>

        <div className="mt-5 text-right text-sm">
          <a
            className="font-medium text-blue-700 underline-offset-4 hover:text-blue-800 hover:underline"
            href={mode === "login" ? "/?auth=register" : "/?auth=login"}
            onClick={() => resetForMode(mode === "login" ? "register" : "login")}
          >
            {mode === "login" ? "还没有账号？" : "已有账号？"}
          </a>
        </div>
      </section>
    </main>
  );
}
