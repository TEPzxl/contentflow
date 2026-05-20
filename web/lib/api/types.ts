export type APIEnvelope<T> = {
  data?: T;
  error?: {
    code: string;
    message: string;
  };
};

export type AuthUser = {
  id: number;
  email: string;
  display_name: string;
};

export type LoginPayload = {
  email: string;
  password: string;
};

export type RegisterPayload = LoginPayload & {
  display_name: string;
};

export type AuthTokens = {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
  user: AuthUser;
};

export type SourceType = "rss" | "email";

export type Source = {
  id: number;
  name: string;
  type: SourceType;
  url: string | null;
  config: Record<string, unknown>;
  is_active: boolean;
  last_fetched_at: string | null;
  last_fetch_status: string;
  last_fetch_message: string;
  created_at: string;
  updated_at: string;
};

export type SourcePayload = {
  name: string;
  type: SourceType;
  url: string | null;
  config: Record<string, unknown>;
};

export type SourceUpdatePayload = {
  name?: string;
  url?: string | null;
  is_active?: boolean;
  config?: Record<string, unknown>;
};

export type Article = {
  id: number;
  source_id: number;
  source_type: string;
  external_id?: string | null;
  title: string;
  url?: string | null;
  original_url?: string | null;
  author: string;
  summary: string;
  content: string;
  published_at?: string | null;
  created_at: string;
  updated_at: string;
  is_read: boolean;
  is_saved: boolean;
  read_at?: string | null;
  saved_at?: string | null;
};

export type CollectionRun = {
  run_id: number;
  source_id: number;
  status: string;
  started_at?: string;
  finished_at?: string | null;
  fetched_count: number;
  inserted_count: number;
  duplicated_count: number;
  error_message: string;
};

export type ListResponse<TItem, TKey extends string> = Record<TKey, TItem[]> & {
  total: number;
  limit: number;
  offset: number;
};

export class APIError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}
