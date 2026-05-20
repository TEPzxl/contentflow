import http from "k6/http";
import { check, group, sleep } from "k6";

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500"]
  }
};

const baseURL = __ENV.BASE_URL || "http://localhost:8080";
const email = __ENV.EMAIL || "";
const password = __ENV.PASSWORD || "";
const configuredToken = __ENV.ACCESS_TOKEN || "";
const sourceID = __ENV.SOURCE_ID || "";
const articleID = __ENV.ARTICLE_ID || "";

export function setup() {
  if (configuredToken || !email || !password) {
    return { token: configuredToken };
  }

  const response = http.post(
    `${baseURL}/api/v1/auth/login`,
    JSON.stringify({ email, password }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(response, {
    "login status is 200": (r) => r.status === 200
  });

  const body = response.json();
  return { token: body?.data?.access_token || "" };
}

export default function (data) {
  const headers = data.token ? { Authorization: `Bearer ${data.token}` } : {};

  group("sources list", () => {
    const response = http.get(`${baseURL}/api/v1/sources?limit=20`, { headers });
    check(response, {
      "sources status is 200 or 401": (r) => r.status === 200 || r.status === 401
    });
  });

  group("articles list", () => {
    const response = http.get(`${baseURL}/api/v1/articles?limit=20`, { headers });
    check(response, {
      "articles status is 200 or 401": (r) => r.status === 200 || r.status === 401
    });
  });

  if (articleID) {
    group("article detail", () => {
      const response = http.get(`${baseURL}/api/v1/articles/${articleID}`, { headers });
      check(response, {
        "article detail status is 200/401/404": (r) => [200, 401, 404].includes(r.status)
      });
    });
  }

  if (sourceID) {
    group("collection runs", () => {
      const response = http.get(`${baseURL}/api/v1/sources/${sourceID}/collection-runs?limit=10`, { headers });
      check(response, {
        "collection runs status is 200/401/404": (r) => [200, 401, 404].includes(r.status)
      });
    });

    if (__ENV.TRIGGER_COLLECT === "1") {
      group("manual collect", () => {
        const response = http.post(`${baseURL}/api/v1/sources/${sourceID}/collect`, null, { headers });
        check(response, {
          "collect status is accepted": (r) => [200, 202, 401, 404, 409, 429].includes(r.status)
        });
      });
    }
  }

  sleep(1);
}
