import { expect, test } from "@playwright/test";

test("visitor can register and is redirected to login without being signed in", async ({
  page,
}) => {
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

    await route.fulfill({
      status: 404,
      body: `Unhandled ${request.method()} ${pathname}`,
    });
  });

  await page.goto("/register");
  await page.getByLabel("Email").fill("  USER@example.com ");
  await page.getByLabel("Password").fill("abc12345");
  await page.getByRole("button", { name: "Create Account" }).click();

  await expect(page).toHaveURL(/\/login\?registered=1$/);
  await expect(
    page.getByText("Account created. Sign in with your new email and password."),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign In" })).toBeVisible();
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page).not.toHaveURL(/\/templates$/);
  await expect(page).not.toHaveURL(/\/admin\/templates$/);
});
