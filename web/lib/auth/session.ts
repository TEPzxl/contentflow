import type { AuthTokens, AuthUser } from "@/lib/api/types";

const accessTokenKey = "contentflow.access_token";
const legacyRefreshTokenKey = "contentflow.refresh_token";
const userKey = "contentflow.user";

export type SessionSnapshot = {
  accessToken: string;
  user: AuthUser | null;
};

export function readSession(): SessionSnapshot | null {
  if (typeof window === "undefined") {
    return null;
  }

  window.localStorage.removeItem(legacyRefreshTokenKey);
  const accessToken = window.sessionStorage.getItem(accessTokenKey);
  const rawUser = window.sessionStorage.getItem(userKey);
  if (!accessToken || !rawUser) {
    return null;
  }

  try {
    return {
      accessToken,
      user: JSON.parse(rawUser) as AuthUser
    };
  } catch {
    clearSession();
    return null;
  }
}

export function saveSession(tokens: AuthTokens): SessionSnapshot {
  window.sessionStorage.setItem(accessTokenKey, tokens.access_token);
  window.sessionStorage.setItem(userKey, JSON.stringify(tokens.user));

  return {
    accessToken: tokens.access_token,
    user: tokens.user
  };
}

export function clearSession() {
  if (typeof window === "undefined") {
    return;
  }
  window.sessionStorage.removeItem(accessTokenKey);
  window.sessionStorage.removeItem(userKey);
  window.localStorage.removeItem(legacyRefreshTokenKey);
}
