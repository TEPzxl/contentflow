import { expect, test } from "@playwright/test";

const corsHeaders = {
  "access-control-allow-credentials": "true",
  "access-control-allow-headers": "content-type, authorization",
  "access-control-allow-methods": "GET,POST,PUT,PATCH,DELETE,OPTIONS",
  "access-control-allow-origin": `http://127.0.0.1:${process.env.CONTENTFLOW_WEB_E2E_PORT ?? "3100"}`
};

test("shows rag errors inside the rag panel", async ({ page }) => {
  await page.route("**/api/v1/auth/login", async (route) => {
    if (route.request().method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers: corsHeaders });
      return;
    }
    await route.fulfill({
      headers: corsHeaders,
      contentType: "application/json",
      body: JSON.stringify({
        data: {
          access_token: "test-token",
          token_type: "Bearer",
          expires_in: 900,
          user: { id: 10, email: "demo@example.com", display_name: "Demo User" }
        }
      })
    });
  });
  await page.route("**/api/v1/sources?*", async (route) => {
    if (route.request().method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers: corsHeaders });
      return;
    }
    await route.fulfill({
      headers: corsHeaders,
      contentType: "application/json",
      body: JSON.stringify({ data: { sources: [], total: 0, limit: 100, offset: 0 } })
    });
  });
  await page.route("**/api/v1/ai/rag-search", async (route) => {
    if (route.request().method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers: corsHeaders });
      return;
    }
    await route.fulfill({
      status: 400,
      headers: corsHeaders,
      contentType: "application/json",
      body: JSON.stringify({
        error: {
          code: "ai_settings_encryption_key_required",
          message: "ai settings encryption key is required"
        }
      })
    });
  });

  await page.goto("/");
  await page.getByLabel("邮箱").fill("demo@example.com");
  await page.getByLabel("密码").fill("password123");
  await page.getByRole("button", { name: "登录" }).click();
  await page.getByRole("button", { name: "AI" }).click();
  await page.getByPlaceholder("输入问题，例如：Kafka 重试失败如何处理").fill("怎么处理重试失败");
  await page.getByRole("button", { name: "提问" }).click();

  const ragPanel = page.locator("section").filter({ has: page.getByRole("heading", { name: "RAG 搜索" }) });
  await expect(ragPanel.getByText("服务端尚未配置 AI 密钥加密 key")).toBeVisible();
});
