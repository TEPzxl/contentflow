import { expect, test, type Page } from "@playwright/test";

const realBackendE2EEnabled = process.env.CONTENTFLOW_E2E_REAL_BACKEND === "1";
const realAPIBaseURL = normalizeURL(process.env.CONTENTFLOW_E2E_API_BASE_URL);
const publicAPIBaseURL = normalizeURL(
  process.env.NEXT_PUBLIC_CONTENTFLOW_API_BASE_URL,
);

function normalizeURL(value: string | undefined) {
  return value?.replace(/\/+$/, "");
}

function requireMatchingRealAPIBaseURL() {
  if (!realBackendE2EEnabled || !realAPIBaseURL) {
    throw new Error(
      "Set CONTENTFLOW_E2E_REAL_BACKEND=1 and CONTENTFLOW_E2E_API_BASE_URL to run against a real backend",
    );
  }

  if (publicAPIBaseURL !== realAPIBaseURL) {
    throw new Error(
      "NEXT_PUBLIC_CONTENTFLOW_API_BASE_URL must match CONTENTFLOW_E2E_API_BASE_URL for real backend E2E tests. " +
        `Got NEXT_PUBLIC_CONTENTFLOW_API_BASE_URL=${publicAPIBaseURL ?? "<unset>"} and ` +
        `CONTENTFLOW_E2E_API_BASE_URL=${realAPIBaseURL}`,
    );
  }

  return realAPIBaseURL;
}

function waitForAPIResponse(page: Page, apiBaseURL: string, path: string) {
  return page.waitForResponse(
    (response) =>
      response.url().startsWith(`${apiBaseURL}${path}`) &&
      response.request().method() === "POST",
    { timeout: 5_000 },
  );
}

test.describe("real backend auth flow", () => {
  test.skip(
    !realBackendE2EEnabled || !realAPIBaseURL,
    "Set CONTENTFLOW_E2E_REAL_BACKEND=1 and CONTENTFLOW_E2E_API_BASE_URL to run against a real backend",
  );

  test("registers, enters the workspace, and logs out through the real API", async ({
    page,
  }) => {
    const apiBaseURL = requireMatchingRealAPIBaseURL();
    const unique = Date.now();
    const email = `e2e-${unique}@example.com`;
    const password = `password-${unique}`;

    await page.goto("/?auth=register");
    await page.getByLabel("显示名称").fill("E2E User");
    await page.getByLabel("邮箱").fill(email);
    await page.getByLabel("密码").fill(password);

    const registerResponse = waitForAPIResponse(
      page,
      apiBaseURL,
      "/auth/register",
    );
    const loginResponse = waitForAPIResponse(page, apiBaseURL, "/auth/login");

    await page.getByRole("button", { name: "注册并登录" }).click();

    const registerResult = await registerResponse;
    expect(
      registerResult.ok(),
      `register request returned ${registerResult.status()}`,
    ).toBeTruthy();

    const loginResult = await loginResponse;
    expect(
      loginResult.ok(),
      `login request returned ${loginResult.status()}`,
    ).toBeTruthy();

    await expect(
      page.getByRole("heading", { name: "内容聚合工作台" }),
    ).toBeVisible();
    await expect(page.getByText(email)).toBeVisible();

    const logoutResponse = waitForAPIResponse(page, apiBaseURL, "/auth/logout");
    await page.getByRole("button", { name: "退出" }).click();

    const logoutResult = await logoutResponse;
    expect(
      logoutResult.ok(),
      `logout request returned ${logoutResult.status()}`,
    ).toBeTruthy();
    await expect(
      page.getByRole("heading", { name: "登录工作台" }),
    ).toBeVisible();
  });
});
