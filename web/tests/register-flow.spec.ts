import { expect, test } from "@playwright/test";

test("visitor requests an email code and must confirm password before registering", async ({
  page,
}) => {
  let registrationRequestCount = 0;
  let sendCodeRequestCount = 0;

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

    if (pathname === "/api/registration/email-codes" && request.method() === "POST") {
      sendCodeRequestCount += 1;
      await route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({ ok: true }),
      });
      return;
    }

    if (pathname === "/api/users" && request.method() === "POST") {
      registrationRequestCount += 1;
      const body = JSON.parse(request.postData() ?? "{}");
      expect(body.emailCode).toBe("123456");
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

    await route.fulfill({ status: 404, body: `Unhandled ${request.method()} ${pathname}` });
  });

  await page.goto("/register");
  await page.getByLabel("邮箱").fill(" USER@example.com ");
  await page.getByRole("button", { name: "发送验证码" }).click();
  await expect(page.getByRole("button", { name: /60 秒后重发|59 秒后重发|重新发送/ })).toBeVisible();

  await page.getByLabel("验证码").fill("123456");
  await page.getByLabel("密码", { exact: true }).fill("abc12345");
  await page.getByLabel("确认密码").fill("abc1234x");
  await page.getByRole("button", { name: "创建账户" }).click();

  await expect(page.getByText("两次输入的密码不一致")).toBeVisible();
  expect(registrationRequestCount).toBe(0);

  await page.getByLabel("确认密码").fill("abc12345");
  await page.getByRole("button", { name: "创建账户" }).click();

  expect(sendCodeRequestCount).toBe(1);
  expect(registrationRequestCount).toBe(1);
  await expect(page).toHaveURL(/\/login\?registered=1$/);
});
