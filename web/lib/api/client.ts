import { apiBaseURL } from "@/lib/config";
import { clearSession, readSession, saveSession } from "@/lib/auth/session";
import type {
  APIEnvelope,
  Article,
  ArticleEmbedding,
  ArticleSummary,
  AuthTokens,
  CollectionRun,
  DailyDigest,
  ListResponse,
  LoginPayload,
  RAGAnswer,
  RegisterPayload,
  SimilarArticle,
  Source,
  SourcePayload,
  SourceUpdatePayload
} from "@/lib/api/types";
import { APIError } from "@/lib/api/types";

type RequestOptions = {
  method?: "GET" | "POST" | "PATCH" | "DELETE";
  body?: unknown;
  auth?: boolean;
  retryOnUnauthorized?: boolean;
};

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers();
  headers.set("Accept", "application/json");
  if (options.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  const session = readSession();
  if (options.auth !== false && session?.accessToken) {
    headers.set("Authorization", `Bearer ${session.accessToken}`);
  }

  const response = await fetch(`${apiBaseURL}${path}`, {
    method: options.method ?? "GET",
    headers,
    credentials: "include",
    body: options.body === undefined ? undefined : JSON.stringify(options.body)
  });

  if (response.status === 401 && options.auth !== false && options.retryOnUnauthorized !== false) {
    const refreshed = await refreshSession();
    if (refreshed) {
      return request<T>(path, { ...options, retryOnUnauthorized: false });
    }
  }

  const payload = (await response.json().catch(() => ({}))) as APIEnvelope<T>;
  if (!response.ok) {
    throw new APIError(
      response.status,
      payload.error?.code ?? "request_failed",
      payload.error?.message ?? "请求失败"
    );
  }

  return payload.data as T;
}

async function refreshSession() {
  const session = readSession();
  if (!session?.accessToken) {
    return false;
  }

  try {
    const data = await request<AuthTokens>("/auth/refresh", {
      method: "POST",
      auth: false,
      retryOnUnauthorized: false
    });
    saveSession(data);
    return true;
  } catch {
    clearSession();
    return false;
  }
}

function withQuery(path: string, params: Record<string, string | number | boolean | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value));
    }
  }
  const query = search.toString();
  return query ? `${path}?${query}` : path;
}

export const api = {
  login(payload: LoginPayload) {
    return request<AuthTokens>("/auth/login", { method: "POST", auth: false, body: payload });
  },
  register(payload: RegisterPayload) {
    return request<{ user: AuthTokens["user"] }>("/auth/register", {
      method: "POST",
      auth: false,
      body: payload
    });
  },
  logout() {
    return request<{ message: string }>("/auth/logout", {
      method: "POST",
      auth: false
    });
  },
  me() {
    return request<{ user: AuthTokens["user"] }>("/me");
  },
  listSources(params: { type?: string; limit?: number; offset?: number } = {}) {
    return request<ListResponse<Source, "sources">>(withQuery("/sources", params));
  },
  createSource(payload: SourcePayload) {
    return request<{ source: Source }>("/sources", { method: "POST", body: payload });
  },
  updateSource(id: number, payload: SourceUpdatePayload) {
    return request<{ source: Source }>(`/sources/${id}`, { method: "PATCH", body: payload });
  },
  collectSource(id: number) {
    return request<{ collection_run: CollectionRun }>(`/sources/${id}/collect`, { method: "POST" });
  },
  listCollectionRuns(sourceID: number, params: { status?: string; limit?: number; offset?: number } = {}) {
    return request<ListResponse<CollectionRun, "collection_runs">>(
      withQuery(`/sources/${sourceID}/collection-runs`, params)
    );
  },
  listArticles(params: {
    source_id?: number;
    q?: string;
    is_read?: boolean;
    is_saved?: boolean;
    limit?: number;
    offset?: number;
  }) {
    return request<ListResponse<Article, "articles">>(withQuery("/articles", params));
  },
  getArticle(id: number) {
    return request<{ article: Article }>(`/articles/${id}`);
  },
  markArticleRead(id: number, isRead: boolean) {
    return request<{ article: Article }>(`/articles/${id}/read`, {
      method: "PATCH",
      body: { is_read: isRead }
    });
  },
  saveArticle(id: number, isSaved: boolean) {
    return request<{ article: Article }>(`/articles/${id}/save`, {
      method: "PATCH",
      body: { is_saved: isSaved }
    });
  },
  requestArticleSummary(id: number, regenerate = false) {
    return request<{ summary: ArticleSummary }>(`/articles/${id}/summary`, {
      method: "POST",
      body: { regenerate }
    });
  },
  getArticleSummary(id: number) {
    return request<{ summary: ArticleSummary }>(`/articles/${id}/summary`);
  },
  generateArticleEmbedding(id: number) {
    return request<{ embedding: ArticleEmbedding }>(`/articles/${id}/embedding`, { method: "POST" });
  },
  listSimilarArticles(id: number, limit = 5) {
    return request<{ articles: SimilarArticle[] }>(withQuery(`/articles/${id}/similar`, { limit }));
  },
  generateDigest(date: string) {
    return request<{ digest: DailyDigest }>(`/ai/digests/${date}`, { method: "POST" });
  },
  getDigest(date: string) {
    return request<{ digest: DailyDigest }>(`/ai/digests/${date}`);
  },
  ragSearch(payload: { query: string; limit?: number }) {
    return request<{ answer: RAGAnswer }>("/ai/rag-search", { method: "POST", body: payload });
  }
};

export function humanizeAPIError(error: unknown) {
  if (!(error instanceof APIError)) {
    return "请求失败，请稍后重试";
  }

  const known: Record<string, string> = {
    unauthorized: "登录已失效，请重新登录",
    invalid_credentials: "邮箱或密码不正确",
    email_already_exists: "该邮箱已注册",
    source_not_found: "来源不存在或无权访问",
    collection_in_progress: "该来源正在采集中，请稍后查看结果",
    collection_run_not_found: "采集记录不存在",
    article_not_found: "文章不存在",
    summary_not_found: "摘要尚未生成",
    embedding_not_found: "向量尚未生成",
    digest_not_found: "日报尚未生成",
    empty_query: "请输入搜索问题",
    rate_limited: "操作过于频繁，请稍后再试"
  };

  return known[error.code] ?? error.message;
}
