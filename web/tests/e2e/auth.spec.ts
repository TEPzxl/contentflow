import { expect, test } from "@playwright/test";

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
