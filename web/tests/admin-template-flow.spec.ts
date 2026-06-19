import { expect, test } from "@playwright/test";
import { mkdtempSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

test("admin can create, edit, add/delete skill, and publish a template", async ({
  page,
}) => {
  let signedIn = false;
  let template: {
    id: string;
    name: string;
    description: string;
    status: "draft" | "published";
    version: number;
    createdAt: string;
    updatedAt: string;
    publishedAt: string | null;
  } = {
    id: "template-0",
    name: "Launch Assistant",
    description: "Initial draft",
    status: "draft",
    version: 1,
    createdAt: "2026-06-15T00:00:00Z",
    updatedAt: "2026-06-15T00:00:00Z",
    publishedAt: null,
  };
  let soul = "# Soul\n";
  let userContent = "# User\n";
  let skills = [
    {
      id: "skill-1",
      templateId: template.id,
      skillName: "triage",
      checksum: "abc123",
      createdAt: "2026-06-15T00:00:00Z",
    },
  ];

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
          user: { id: "admin-1", email: "admin@example.com", role: "admin" },
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
          user: { id: "admin-1", email: "admin@example.com", role: "admin" },
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

    if (pathname === "/api/admin/templates" && request.method() === "POST") {
      const contentType = request.headers()["content-type"] ?? "";
      expect(contentType).toContain("multipart/form-data");
      template = {
        ...template,
        id: "template-1",
        name: "Support Coach",
        description: "Helps ops teams triage requests.",
      };
      soul = "# Soul\nYou are calm and direct.";
      userContent = "# User\nPrefer concise answers.";
      skills = [
        {
          id: "skill-1",
          templateId: "template-1",
          skillName: "triage",
          checksum: "checksum-1",
          createdAt: "2026-06-15T00:00:00Z",
        },
      ];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}` && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}` && request.method() === "PUT") {
      const body = JSON.parse(request.postData() ?? "{}");
      template = { ...template, name: body.name, description: body.description };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}` && request.method() === "DELETE") {
      await route.fulfill({ status: 204 });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}/soul`) {
      if (request.method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ content: soul }),
        });
        return;
      }
      soul = JSON.parse(request.postData() ?? "{}").content;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}/user`) {
      if (request.method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ content: userContent }),
        });
        return;
      }
      userContent = JSON.parse(request.postData() ?? "{}").content;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}/skills`) {
      if (request.method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ skills }),
        });
        return;
      }
      const body = await request.postDataBuffer();
      expect(request.headers()["content-type"] ?? "").toContain("multipart/form-data");
      expect(body?.length ?? 0).toBeGreaterThan(0);
      const skill = {
        id: "skill-2",
        templateId: template.id,
        skillName: "handoff",
        checksum: "def456",
        createdAt: "2026-06-15T00:00:00Z",
      };
      skills = [...skills, skill];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({ skill }),
      });
      return;
    }

    if (
      pathname === `/api/admin/templates/${template.id}/skills/skill-2` &&
      request.method() === "DELETE"
    ) {
      skills = skills.filter((skill) => skill.id !== "skill-2");
      await route.fulfill({ status: 204 });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}/publication`) {
      template = {
        ...template,
        status: request.method() === "PUT" ? "published" : "draft",
        publishedAt:
          request.method() === "PUT" ? "2026-06-15T00:00:00Z" : null,
      };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    await route.fulfill({ status: 404, body: `Unhandled ${request.method()} ${pathname}` });
  });

  await page.goto("/login");
  await page.getByLabel("Email").fill("admin@example.com");
  await page.getByLabel("Password").fill("secret");
  await page.getByRole("button", { name: "Sign In" }).click();

  await expect(page).toHaveURL(/\/admin\/templates$/);
  await page.getByRole("link", { name: "New Draft" }).click();
  await page.getByLabel("Name").fill("Support Coach");
  await page.getByLabel("Description").fill("Helps ops teams triage requests.");
  await page.getByLabel("SOUL.md").fill("# Soul\nYou are calm and direct.");
  await page.getByLabel("USER.md").fill("# User\nPrefer concise answers.");
  await page.getByLabel("Skill ZIPs").setInputFiles({
    name: "triage.zip",
    mimeType: "application/zip",
    buffer: Buffer.from("fake-zip"),
  });
  await page.getByRole("button", { name: "Create Draft" }).click();

  await expect(page).toHaveURL(/\/admin\/templates\/template-1$/);
  await expect(page.getByText("triage", { exact: true })).toBeVisible();
  await page.getByRole("textbox").nth(2).fill("# Soul\nYou are calm and direct.");
  await page.getByRole("button", { name: "Save SOUL" }).click();
  await page.getByRole("textbox").nth(3).fill("# User\nPrefer concise answers.");
  await page.getByRole("button", { name: "Save USER" }).click();

  const skillZip = join(mkdtempSync(join(tmpdir(), "agentforge-skill-")), "handoff.zip");
  writeFileSync(skillZip, "placeholder");
  await page.getByLabel("Skill ZIP").setInputFiles(skillZip);
  await page.getByRole("button", { name: "Upload Skill" }).click();
  await expect(page.getByText("handoff")).toBeVisible();
  await expect(page.getByRole("button", { name: /Edit skill/i })).toHaveCount(0);

  await page.getByRole("button", { name: "Delete" }).last().click();
  await expect(page.getByText("handoff")).toHaveCount(0);

  await page.getByRole("button", { name: "Publish" }).click();
  await expect(page.getByRole("button", { name: "Unpublish" })).toBeVisible();

  await page.getByRole("button", { name: "Delete Template" }).click();
  await page.getByRole("button", { name: "Confirm Delete" }).click();
  await expect(page).toHaveURL(/\/admin\/templates$/);
  await expect(page.getByRole("link", { name: "Support Coach" })).toHaveCount(0);
});

test("publish is disabled while skill upload is in progress", async ({ page }) => {
  let signedIn = false;
  let template = {
    id: "template-1",
    name: "Support Coach",
    description: "Helps ops teams triage requests.",
    status: "draft" as const,
    version: 1,
    createdAt: "2026-06-15T00:00:00Z",
    updatedAt: "2026-06-15T00:00:00Z",
    publishedAt: null as string | null,
  };
  const soul = "# Soul\nYou are calm and direct.";
  const userContent = "# User\nPrefer concise answers.";
  const skills = [
    {
      id: "skill-1",
      templateId: template.id,
      skillName: "triage",
      checksum: "abc123",
      createdAt: "2026-06-15T00:00:00Z",
    },
  ];

  let releaseUpload: (() => void) | null = null;
  const uploadBlocked = new Promise<void>((resolve) => {
    releaseUpload = resolve;
  });

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;

    if (pathname === "/api/session" && request.method() === "GET") {
      await route.fulfill({
        status: signedIn ? 200 : 401,
        contentType: "application/json",
        body: JSON.stringify(
          signedIn
            ? { user: { id: "admin-1", email: "admin@example.com", role: "admin" } }
            : { error: "unauthorized" },
        ),
      });
      return;
    }

    if (pathname === "/api/sessions" && request.method() === "POST") {
      signedIn = true;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "admin-1", email: "admin@example.com", role: "admin" },
        }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}` && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}/soul` && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ content: soul }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}/user` && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ content: userContent }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}/skills`) {
      if (request.method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ skills }),
        });
        return;
      }
      await uploadBlocked;
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          skill: {
            id: "skill-2",
            templateId: template.id,
            skillName: "handoff",
            checksum: "def456",
            createdAt: "2026-06-15T00:00:00Z",
          },
        }),
      });
      return;
    }

    if (pathname === `/api/admin/templates/${template.id}/publication`) {
      template = {
        ...template,
        status: "published",
        publishedAt: "2026-06-15T00:00:00Z",
      };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ template }),
      });
      return;
    }

    await route.fulfill({ status: 404, body: `Unhandled ${request.method()} ${pathname}` });
  });

  await page.goto("/login");
  await page.getByLabel("Email").fill("admin@example.com");
  await page.getByLabel("Password").fill("secret");
  await page.getByRole("button", { name: "Sign In" }).click();

  await page.goto("/admin/templates/template-1");
  await expect(page.getByRole("button", { name: "Publish" })).toBeEnabled();

  const skillZip = join(mkdtempSync(join(tmpdir(), "agentforge-skill-")), "handoff.zip");
  writeFileSync(skillZip, "placeholder");
  await page.getByLabel("Skill ZIP").setInputFiles(skillZip);
  await page.getByRole("button", { name: "Upload Skill" }).click();

  await expect(page.getByRole("button", { name: "Publishing..." })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Publish" })).toBeDisabled();

  releaseUpload?.();

  await expect(page.getByText("handoff")).toBeVisible();
  await expect(page.getByRole("button", { name: "Publish" })).toBeEnabled();
});
