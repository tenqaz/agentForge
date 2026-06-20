import { expect, test } from "@playwright/test";

test("admin can log in via /admin/login and non-admin is rejected", async ({ page }) => {
  let signedInRole: "admin" | "user" | null = null;

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;

    if (pathname === "/api/session" && request.method() === "GET") {
      if (!signedInRole) {
        await route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({ error: "unauthorized" }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "u1", email: "admin@agentforge.dev", role: signedInRole },
        }),
      });
      return;
    }

    if (pathname === "/api/sessions" && request.method() === "POST") {
      // 根据邮箱决定角色：admin@ 为管理员，其它为普通用户
      const body = request.postDataJSON() as { email: string };
      signedInRole = body.email.startsWith("admin@") ? "admin" : "user";
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "u1", email: body.email, role: signedInRole },
        }),
      });
      return;
    }

    if (pathname === "/api/admin/templates" && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ templates: [] }),
      });
      return;
    }

    await route.fulfill({ status: 404, body: `Unhandled ${request.method()} ${pathname}` });
  });

  // 管理员登录成功跳转
  await page.goto("/admin/login");
  await expect(page.getByRole("heading", { name: "管理员登录" })).toBeVisible();
  await page.getByLabel("邮箱").fill("admin@agentforge.dev");
  await page.getByLabel("密码").fill("secret");
  await page.getByRole("button", { name: "登录管理后台" }).click();
  await expect(page).toHaveURL(/\/admin\/templates$/);

  // 非管理员登录被拒（停留在 /admin/login 且显示错误）
  signedInRole = null;
  await page.goto("/admin/login");
  await page.getByLabel("邮箱").fill("user@example.com");
  await page.getByLabel("密码").fill("secret");
  await page.getByRole("button", { name: "登录管理后台" }).click();
  await expect(page.getByText(/不是管理员/)).toBeVisible();
  await expect(page).toHaveURL(/\/admin\/login$/);
});
