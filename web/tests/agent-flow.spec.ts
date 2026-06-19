import { expect, test } from "@playwright/test";

test("user can create an agent and complete mocked Weixin pairing", async ({
  page,
}) => {
  let signedIn = false;
  let runtimePolls = 0;
  let pairingPolls = 0;
  let agentCreated = false;
  let channelStatus = "not_configured";

  const template = {
    id: "template-1",
    name: "Support Concierge",
    description: "Answers private support questions.",
    status: "published",
    version: 3,
    createdAt: "2026-06-15T00:00:00Z",
    updatedAt: "2026-06-15T00:00:00Z",
    publishedAt: "2026-06-15T00:00:00Z",
  };

  const agent = {
    id: "agent-1",
    ownerUserId: "user-1",
    templateId: template.id,
    templateVersion: template.version,
    name: "Support Concierge Agent",
    status: "creating",
    runtimeId: "",
    lastErrorCode: "",
    lastErrorMessage: "",
    createdAt: "2026-06-15T00:00:00Z",
    updatedAt: "2026-06-15T00:00:00Z",
  };

  const runtime = () => ({
    agentId: agent.id,
    runtimeId: runtimePolls >= 2 ? "agentforge-hermes-agent-1" : "",
    status: runtimePolls >= 2 ? "running" : "starting",
    lastErrorCode: "",
    lastErrorMessage: "",
    updatedAt: "2026-06-15T00:00:00Z",
  });

  const session = () => ({
    id: "pairing-1",
    status: pairingPolls >= 2 ? "connected" : "pending",
    qrPayload: "weixin://pairing/qr-1",
    qrPayloadUrl: "https://liteapp.weixin.qq.com/q/test?qrcode=abc123",
    expiresAt: "2026-06-15T00:05:00Z",
  });

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;

    if (pathname === "/api/session" && request.method() === "GET") {
      if (!signedIn) {
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
          user: { id: "user-1", email: "user@example.com", role: "user" },
        }),
      });
      return;
    }

    if (pathname === "/api/sessions" && request.method() === "POST") {
      signedIn = true;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "user-1", email: "user@example.com", role: "user" },
        }),
      });
      return;
    }

    if (pathname === "/api/templates" && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ templates: [template] }),
      });
      return;
    }

    if (pathname === `/api/templates/${template.id}` && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    if (pathname === "/api/agents" && request.method() === "POST") {
      agentCreated = true;
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          agent: { ...agent, name: "Support Concierge Agent" },
        }),
      });
      return;
    }

    if (pathname === "/api/agents" && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ agents: agentCreated ? [agent] : [] }),
      });
      return;
    }

    if (pathname === `/api/agents/${agent.id}` && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ agent }),
      });
      return;
    }

    if (pathname === `/api/agents/${agent.id}/runtime` && request.method() === "GET") {
      runtimePolls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ runtime: runtime() }),
      });
      return;
    }

    if (
      pathname === `/api/agents/${agent.id}/channels/weixin` &&
      request.method() === "GET"
    ) {
      if (pairingPolls >= 2) {
        channelStatus = "connected";
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          channel: {
            channelType: "weixin",
            status: channelStatus,
            externalAccountId: pairingPolls >= 2 ? "wx-bot-1" : "",
          },
        }),
      });
      return;
    }

    if (
      pathname === `/api/agents/${agent.id}/channels/weixin` &&
      request.method() === "PUT"
    ) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          channel: { channelType: "weixin", status: "not_configured" },
        }),
      });
      return;
    }

    if (
      pathname === `/api/agents/${agent.id}/channels/weixin/pairing-sessions` &&
      request.method() === "POST"
    ) {
      channelStatus = "qr_pending";
      pairingPolls = 0;
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({ session: session() }),
      });
      return;
    }

    if (
      pathname === `/api/agents/${agent.id}/channels/weixin/pairing-sessions` &&
      request.method() === "GET"
    ) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ sessions: [session()] }),
      });
      return;
    }

    if (
      pathname ===
        `/api/agents/${agent.id}/channels/weixin/pairing-sessions/pairing-1` &&
      request.method() === "GET"
    ) {
      pairingPolls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ session: session() }),
      });
      return;
    }

    await route.fulfill({ status: 404, body: `Unhandled ${request.method()} ${pathname}` });
  });

  await page.goto("/login");
  await page.getByLabel("邮箱").fill("user@example.com");
  await page.getByLabel("密码").fill("secret");
  await page.getByRole("button", { name: "登录" }).click();

  await expect(page).toHaveURL(/\/templates$/);
  await expect(page.getByText("Support Concierge")).toBeVisible();
  await page.getByRole("link", { name: /Support Concierge/i }).click();

  await page.getByRole("button", { name: "创建 Agent" }).click();
  await expect(page).toHaveURL(/\/agents\/agent-1$/);
  await expect(page.getByText("运行中")).toBeVisible();
  await expect(page.getByRole("button", { name: "生成配对" })).toBeEnabled();

  await page.getByRole("button", { name: "生成配对" }).click();
  await expect(page.getByAltText("微信配对二维码")).toBeVisible();
  await expect(page.getByRole("heading", { name: "已连接" })).toBeVisible();
  await expect(page.getByText("关联账号 wx-bot-1")).toBeVisible();
});
