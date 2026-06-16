import { expect, test } from "@playwright/test";
import { mkdtempSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

test("admin can create, edit, add/delete skill, and publish a template", async ({
  page,
}) => {
  let signedIn = false;
  let template = {
    id: "template-1",
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
        body: JSON.stringify({ templates: [template] }),
      });
      return;
    }

    if (pathname === "/api/admin/templates" && request.method() === "POST") {
      const body = JSON.parse(request.postData() ?? "{}");
      template = { ...template, name: body.name, description: body.description };
      await route.fulfill({
        status: 201,
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
  await page.getByRole("button", { name: "Create Draft" }).click();

  await expect(page).toHaveURL(/\/admin\/templates\/template-1$/);
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
});
