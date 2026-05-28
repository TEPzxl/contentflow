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
