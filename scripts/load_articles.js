import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500"]
  }
};

const baseURL = __ENV.BASE_URL || "http://localhost:8080";
const token = __ENV.ACCESS_TOKEN || "";

export default function () {
  const response = http.get(`${baseURL}/api/v1/articles?limit=20`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {}
  });

  check(response, {
    "status is 200 or 401": (r) => r.status === 200 || r.status === 401
  });
  sleep(1);
}
