import { expect, test } from "@playwright/test";

test("restores workspace from refresh cookie when session storage is empty", async ({
  page,
}) => {
  await page.route("**/api/v1/auth/refresh", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      json: {
        data: {
          access_token: "restored-access-token",
          token_type: "Bearer",
          expires_in: 900,
          user: {
            id: 100,
            email: "restored@example.com",
            display_name: "Restored User",
          },
        },
      },
    });
  });
  await page.route("**/api/v1/sources?*", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      json: {
        data: {
          sources: [],
          total: 0,
          limit: 100,
          offset: 0,
        },
      },
    });
  });

  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "内容聚合工作台" }),
  ).toBeVisible();
  await expect(page.getByText("restored@example.com")).toBeVisible();
  await expect(page.getByRole("heading", { name: "登录工作台" })).toBeHidden();
});

test("shows authentication workspace entry", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "登录工作台" })).toBeVisible();
  await expect(page.getByLabel("邮箱")).toBeVisible();
  await expect(page.getByLabel("密码")).toBeVisible();

  await page.getByLabel("邮箱").fill("demo@example.com");
  await page.getByLabel("密码").fill("password123");

  await page.getByRole("link", { name: "还没有账号？" }).click();
  await expect(page.getByRole("heading", { name: "创建账号" })).toBeVisible();
  await expect(page.getByLabel("显示名称")).toBeVisible();
  await expect(page.getByLabel("邮箱")).toHaveValue("");
  await expect(page.getByLabel("密码")).toHaveValue("");
});

test("supports direct register entry and clears fields when returning to login", async ({ page }) => {
  await page.goto("/?auth=register");

  await expect(page.getByRole("heading", { name: "创建账号" })).toBeVisible();
  await page.getByLabel("显示名称").fill("Demo User");
  await page.getByLabel("邮箱").fill("demo@example.com");
  await page.getByLabel("密码").fill("password123");

  await page.getByRole("link", { name: "已有账号？" }).click();
  await expect(page.getByRole("heading", { name: "登录工作台" })).toBeVisible();
  await expect(page.getByLabel("邮箱")).toHaveValue("");
  await expect(page.getByLabel("密码")).toHaveValue("");
  await expect(page.getByLabel("显示名称")).toBeHidden();
});
