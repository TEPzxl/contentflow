import type { AuthTokens, AuthUser } from "@/lib/api/types";

const accessTokenKey = "contentflow.access_token";
const refreshTokenKey = "contentflow.refresh_token";
const userKey = "contentflow.user";

export type SessionSnapshot = {
  accessToken: string;
  refreshToken: string;
  user: AuthUser | null;
};

export function readSession(): SessionSnapshot | null {
  if (typeof window === "undefined") {
    return null;
  }

  const accessToken = window.localStorage.getItem(accessTokenKey);
  const refreshToken = window.localStorage.getItem(refreshTokenKey);
  const rawUser = window.localStorage.getItem(userKey);
  if (!accessToken || !refreshToken || !rawUser) {
    return null;
  }

  try {
    return {
      accessToken,
      refreshToken,
      user: JSON.parse(rawUser) as AuthUser
    };
  } catch {
    clearSession();
    return null;
  }
}

export function saveSession(tokens: AuthTokens): SessionSnapshot {
  window.localStorage.setItem(accessTokenKey, tokens.access_token);
  window.localStorage.setItem(refreshTokenKey, tokens.refresh_token);
  window.localStorage.setItem(userKey, JSON.stringify(tokens.user));

  return {
    accessToken: tokens.access_token,
    refreshToken: tokens.refresh_token,
    user: tokens.user
  };
}

export function clearSession() {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(accessTokenKey);
  window.localStorage.removeItem(refreshTokenKey);
  window.localStorage.removeItem(userKey);
}
