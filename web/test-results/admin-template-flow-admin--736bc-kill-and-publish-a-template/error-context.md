# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: admin-template-flow.spec.ts >> admin can create, edit, add/delete skill, and publish a template
- Location: tests/admin-template-flow.spec.ts:3:5

# Error details

```
Error: page.goto: net::ERR_CONNECTION_REFUSED at http://127.0.0.1:3007/login
Call log:
  - navigating to "http://127.0.0.1:3007/login", waiting until "load"

```

# Test source

```ts
  125 |         contentType: "application/json",
  126 |         body: JSON.stringify({ template }),
  127 |       });
  128 |       return;
  129 |     }
  130 | 
  131 |     if (pathname === `/api/admin/templates/${template.id}` && request.method() === "DELETE") {
  132 |       await route.fulfill({ status: 204 });
  133 |       return;
  134 |     }
  135 | 
  136 |     if (pathname === `/api/admin/templates/${template.id}/soul`) {
  137 |       if (request.method() === "GET") {
  138 |         await route.fulfill({
  139 |           status: 200,
  140 |           contentType: "application/json",
  141 |           body: JSON.stringify({ content: soul }),
  142 |         });
  143 |         return;
  144 |       }
  145 |       soul = JSON.parse(request.postData() ?? "{}").content;
  146 |       await route.fulfill({
  147 |         status: 200,
  148 |         contentType: "application/json",
  149 |         body: JSON.stringify({ template }),
  150 |       });
  151 |       return;
  152 |     }
  153 | 
  154 |     if (pathname === `/api/admin/templates/${template.id}/user`) {
  155 |       if (request.method() === "GET") {
  156 |         await route.fulfill({
  157 |           status: 200,
  158 |           contentType: "application/json",
  159 |           body: JSON.stringify({ content: userContent }),
  160 |         });
  161 |         return;
  162 |       }
  163 |       userContent = JSON.parse(request.postData() ?? "{}").content;
  164 |       await route.fulfill({
  165 |         status: 200,
  166 |         contentType: "application/json",
  167 |         body: JSON.stringify({ template }),
  168 |       });
  169 |       return;
  170 |     }
  171 | 
  172 |     if (pathname === `/api/admin/templates/${template.id}/skills`) {
  173 |       if (request.method() === "GET") {
  174 |         await route.fulfill({
  175 |           status: 200,
  176 |           contentType: "application/json",
  177 |           body: JSON.stringify({ skills }),
  178 |         });
  179 |         return;
  180 |       }
  181 |       const body = JSON.parse(request.postData() ?? "{}");
  182 |       const skill = {
  183 |         id: "skill-2",
  184 |         templateId: template.id,
  185 |         skillName: body.skillName,
  186 |         checksum: "def456",
  187 |         createdAt: "2026-06-15T00:00:00Z",
  188 |       };
  189 |       skills = [...skills, skill];
  190 |       await route.fulfill({
  191 |         status: 201,
  192 |         contentType: "application/json",
  193 |         body: JSON.stringify({ skill }),
  194 |       });
  195 |       return;
  196 |     }
  197 | 
  198 |     if (
  199 |       pathname === `/api/admin/templates/${template.id}/skills/skill-2` &&
  200 |       request.method() === "DELETE"
  201 |     ) {
  202 |       skills = skills.filter((skill) => skill.id !== "skill-2");
  203 |       await route.fulfill({ status: 204 });
  204 |       return;
  205 |     }
  206 | 
  207 |     if (pathname === `/api/admin/templates/${template.id}/publication`) {
  208 |       template = {
  209 |         ...template,
  210 |         status: request.method() === "PUT" ? "published" : "draft",
  211 |         publishedAt:
  212 |           request.method() === "PUT" ? "2026-06-15T00:00:00Z" : null,
  213 |       };
  214 |       await route.fulfill({
  215 |         status: 200,
  216 |         contentType: "application/json",
  217 |         body: JSON.stringify({ template }),
  218 |       });
  219 |       return;
  220 |     }
  221 | 
  222 |     await route.fulfill({ status: 404, body: `Unhandled ${request.method()} ${pathname}` });
  223 |   });
  224 | 
> 225 |   await page.goto("/login");
      |              ^ Error: page.goto: net::ERR_CONNECTION_REFUSED at http://127.0.0.1:3007/login
  226 |   await page.getByLabel("Email").fill("admin@example.com");
  227 |   await page.getByLabel("Password").fill("secret");
  228 |   await page.getByRole("button", { name: "Sign In" }).click();
  229 | 
  230 |   await expect(page).toHaveURL(/\/admin\/templates$/);
  231 |   await page.getByRole("link", { name: "New Draft" }).click();
  232 |   await page.getByLabel("Name").fill("Support Coach");
  233 |   await page.getByLabel("Description").fill("Helps ops teams triage requests.");
  234 |   await page.getByLabel("SOUL.md").fill("# Soul\nYou are calm and direct.");
  235 |   await page.getByLabel("USER.md").fill("# User\nPrefer concise answers.");
  236 |   await page.getByLabel("Skill ZIPs").setInputFiles({
  237 |     name: "triage.zip",
  238 |     mimeType: "application/zip",
  239 |     buffer: Buffer.from("fake-zip"),
  240 |   });
  241 |   await page.getByRole("button", { name: "Create Draft" }).click();
  242 | 
  243 |   await expect(page).toHaveURL(/\/admin\/templates\/template-1$/);
  244 |   await expect(page.getByText("triage")).toBeVisible();
  245 |   await page.getByRole("textbox").nth(2).fill("# Soul\nYou are calm and direct.");
  246 |   await page.getByRole("button", { name: "Save SOUL" }).click();
  247 |   await page.getByRole("textbox").nth(3).fill("# User\nPrefer concise answers.");
  248 |   await page.getByRole("button", { name: "Save USER" }).click();
  249 | 
  250 |   await page.getByLabel("skill_name").fill("handoff");
  251 |   await page.getByLabel("SKILL.md").fill("# SKILL\nEscalate to humans when needed.");
  252 |   await page.getByRole("button", { name: "Add Skill" }).click();
  253 |   await expect(page.getByText("handoff")).toBeVisible();
  254 |   await expect(page.getByRole("button", { name: /Edit skill/i })).toHaveCount(0);
  255 | 
  256 |   await page.getByRole("button", { name: "Delete" }).last().click();
  257 |   await expect(page.getByText("handoff")).toHaveCount(0);
  258 | 
  259 |   await page.getByRole("button", { name: "Publish" }).click();
  260 |   await expect(page.getByRole("button", { name: "Unpublish" })).toBeVisible();
  261 | 
  262 |   await page.getByRole("button", { name: "Delete Template" }).click();
  263 |   await page.getByRole("button", { name: "Confirm Delete" }).click();
  264 |   await expect(page).toHaveURL(/\/admin\/templates$/);
  265 |   await expect(page.getByRole("link", { name: "Support Coach" })).toHaveCount(0);
  266 | });
  267 | 
```