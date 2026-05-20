export const apiBaseURL =
  process.env.NEXT_PUBLIC_CONTENTFLOW_API_BASE_URL?.replace(/\/$/, "") ||
  "http://localhost:8080/api/v1";
