import { expect, test, type Page, type Route } from "@playwright/test";

const corsHeaders = {
  "access-control-allow-credentials": "true",
  "access-control-allow-headers": "content-type, authorization",
  "access-control-allow-methods": "GET,POST,PUT,PATCH,DELETE,OPTIONS",
  "access-control-allow-origin": `http://127.0.0.1:${process.env.CONTENTFLOW_WEB_E2E_PORT ?? "3100"}`,
};

async function fulfillOptions(route: Route) {
  if (route.request().method() === "OPTIONS") {
    await route.fulfill({ status: 204, headers: corsHeaders });
    return true;
  }
  return false;
}

async function mockAuthenticatedWorkspace(page: Page) {
  await page.addInitScript(() => {
    window.sessionStorage.setItem("contentflow.access_token", "test-token");
    window.sessionStorage.setItem(
      "contentflow.user",
      JSON.stringify({
        id: 10,
        email: "demo@example.com",
        display_name: "Demo User",
      }),
    );
  });

  await page.route("**/api/v1/sources?*", async (route) => {
    if (await fulfillOptions(route)) {
      return;
    }
    await route.fulfill({
      headers: corsHeaders,
      contentType: "application/json",
      body: JSON.stringify({
        data: { sources: [], total: 0, limit: 100, offset: 0 },
      }),
    });
  });

  await page.route("**/api/v1/articles?*", async (route) => {
    if (await fulfillOptions(route)) {
      return;
    }
    await route.fulfill({
      headers: corsHeaders,
      contentType: "application/json",
      body: JSON.stringify({
        data: { articles: [], total: 0, limit: 20, offset: 0 },
      }),
    });
  });
}

async function openAIPanel(page: Page) {
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "内容聚合工作台" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "AI" }).click();
}

function ragPanel(page: Page) {
  return page
    .locator("section")
    .filter({ has: page.getByRole("heading", { name: "RAG 搜索" }) });
}

function digestPanel(page: Page) {
  return page
    .locator("section")
    .filter({ has: page.getByRole("heading", { name: "Daily Digest" }) });
}

test("shows rag errors inside the rag panel", async ({ page }) => {
  await mockAuthenticatedWorkspace(page);
  await page.route("**/api/v1/ai/rag-search", async (route) => {
    if (await fulfillOptions(route)) {
      return;
    }
    await route.fulfill({
      status: 400,
      headers: corsHeaders,
      contentType: "application/json",
      body: JSON.stringify({
        error: {
          code: "ai_settings_encryption_key_required",
          message: "ai settings encryption key is required",
        },
      }),
    });
  });

  await openAIPanel(page);
  await page
    .getByPlaceholder("输入问题，例如：Kafka 重试失败如何处理")
    .fill("怎么处理重试失败");
  await page.getByRole("button", { name: "提问" }).click();

  await expect(
    ragPanel(page).getByText("服务端尚未配置 AI 密钥加密 key"),
  ).toBeVisible();
});

test("clears stale rag answer when the next request fails", async ({
  page,
}) => {
  await mockAuthenticatedWorkspace(page);
  let ragRequests = 0;
  await page.route("**/api/v1/ai/rag-search", async (route) => {
    if (await fulfillOptions(route)) {
      return;
    }
    ragRequests += 1;
    if (ragRequests === 1) {
      await route.fulfill({
        headers: corsHeaders,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            answer: {
              model: "test-model",
              prompt_version: "rag-v1",
              answer: "这是上一轮成功答案",
              citations: [
                {
                  article_id: 1,
                  title: "重试策略",
                  snippet: "旧引用",
                  url: null,
                },
              ],
            },
          },
        }),
      });
      return;
    }
    await route.fulfill({
      status: 400,
      headers: corsHeaders,
      contentType: "application/json",
      body: JSON.stringify({
        error: {
          code: "ai_settings_encryption_key_required",
          message: "ai settings encryption key is required",
        },
      }),
    });
  });

  await openAIPanel(page);
  await page
    .getByPlaceholder("输入问题，例如：Kafka 重试失败如何处理")
    .fill("第一轮问题");
  await page.getByRole("button", { name: "提问" }).click();
  await expect(ragPanel(page).getByText("这是上一轮成功答案")).toBeVisible();

  await page
    .getByPlaceholder("输入问题，例如：Kafka 重试失败如何处理")
    .fill("第二轮问题");
  await page.getByRole("button", { name: "提问" }).click();

  await expect(
    ragPanel(page).getByText("服务端尚未配置 AI 密钥加密 key"),
  ).toBeVisible();
  await expect(ragPanel(page).getByText("这是上一轮成功答案")).toBeHidden();
});

test("clears stale digest when the next digest request fails", async ({
  page,
}) => {
  await mockAuthenticatedWorkspace(page);
  let digestRequests = 0;
  await page.route("**/api/v1/ai/digests/*", async (route) => {
    if (await fulfillOptions(route)) {
      return;
    }
    digestRequests += 1;
    if (digestRequests === 1) {
      await route.fulfill({
        headers: corsHeaders,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            digest: {
              id: 1,
              digest_date: "2026-05-29",
              model: "test-model",
              prompt_version: "digest-v1",
              summary: "这是上一轮成功日报",
              article_ids: [1],
              status: "succeeded",
              error_message: "",
              created_at: "2026-05-29T00:00:00Z",
              updated_at: "2026-05-29T00:00:00Z",
            },
          },
        }),
      });
      return;
    }
    await route.fulfill({
      status: 404,
      headers: corsHeaders,
      contentType: "application/json",
      body: JSON.stringify({
        error: {
          code: "digest_not_found",
          message: "digest not found",
        },
      }),
    });
  });

  await openAIPanel(page);
  await page.getByRole("button", { name: "生成" }).click();
  await expect(digestPanel(page).getByText("这是上一轮成功日报")).toBeVisible();

  await page.getByRole("button", { name: "读取" }).click();

  await expect(digestPanel(page).getByText("日报尚未生成")).toBeVisible();
  await expect(digestPanel(page).getByText("这是上一轮成功日报")).toBeHidden();
});
