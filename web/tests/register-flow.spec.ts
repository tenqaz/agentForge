import { expect, test } from "@playwright/test";

test("visitor can register and is redirected to login without being signed in", async ({
  page,
}) => {
  let loginAttemptCount = 0;
  let registrationRequestCount = 0;

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;

    if (pathname === "/api/session" && request.method() === "GET") {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "unauthorized" }),
      });
      return;
    }

    if (pathname === "/api/users" && request.method() === "POST") {
      registrationRequestCount += 1;
      const body = JSON.parse(request.postData() ?? "{}");
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "user-1",
            email: body.email.trim().toLowerCase(),
            role: "user",
          },
        }),
      });
      return;
    }

    if (pathname === "/api/sessions" && request.method() === "POST") {
      loginAttemptCount += 1;
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "invalid credentials" }),
      });
      return;
    }

    await route.fulfill({
      status: 404,
      body: `Unhandled ${request.method()} ${pathname}`,
    });
  });

  await page.goto("/register");
  await page.getByLabel("邮箱").fill("  USER@example.com ");
  await page.getByLabel("密码").fill("abc12345");
  await page.getByRole("button", { name: "创建账户" }).click();

  await expect(page).toHaveURL(/\/login\?registered=1$/);
  await expect(
    page.getByText("账户已创建，请使用新邮箱和密码登录"),
  ).toBeVisible();
  expect(registrationRequestCount).toBe(1);
  expect(loginAttemptCount).toBe(0);
  await expect(page.getByRole("button", { name: "登录" })).toBeVisible();
  await expect(page.getByLabel("邮箱")).toBeVisible();
  await expect(page).not.toHaveURL(/\/templates$/);
  await expect(page).not.toHaveURL(/\/admin\/templates$/);
});
