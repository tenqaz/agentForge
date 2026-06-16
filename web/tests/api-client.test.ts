import { describe, expect, it, vi } from "vitest";

import {
  archiveAdminTemplate,
  createAdminTemplate,
  createApiClient,
  registerUser,
} from "@/lib/api";

describe("createApiClient", () => {
  it("maps backend error payloads that only expose error codes", async () => {
    const fetchImpl: typeof fetch = vi.fn(async () =>
      new Response(JSON.stringify({ error: "invalid_credentials" }), {
        status: 401,
        headers: { "content-type": "application/json" },
      }),
    );
    const client = createApiClient({ fetchImpl });

    const response = await client.post("/api/sessions", {
      email: "user@example.com",
      password: "wrong",
    });

    expect(response.ok).toBe(false);
    if (response.ok) {
      throw new Error("expected error response");
    }
    expect(response.error.code).toBe("invalid_credentials");
    expect(response.error.message).toContain("invalid credentials");
  });

  it("serializes JSON bodies and supports 204 responses", async () => {
    const fetchImpl: typeof fetch = vi.fn(async (_input, init?: RequestInit) => {
      expect(init?.method).toBe("DELETE");
      expect(init?.headers).toBeDefined();
      return new Response(null, { status: 204 });
    });
    const client = createApiClient({ fetchImpl, baseUrl: "http://example.test" });

    const response = await client.delete("/api/admin/templates/template-1");

    expect(response.ok).toBe(true);
    if (!response.ok) {
      throw new Error("expected success response");
    }
    expect(response.status).toBe(204);
  });

  it("posts registration payloads and preserves signup conflict codes", async () => {
    const fetchImpl: typeof fetch = vi.fn(async (input, init?: RequestInit) => {
      const url = typeof input === "string" ? input : String(input);
      expect(url).toBe("http://example.test/api/users");
      expect(init?.method).toBe("POST");
      expect(init?.headers).toBeDefined();
      expect(init?.body).toBe(
        JSON.stringify({
          email: "user@example.com",
          password: "abc12345",
        }),
      );
      return new Response(JSON.stringify({ error: "email_already_exists" }), {
        status: 409,
        headers: { "content-type": "application/json" },
      });
    });
    const client = createApiClient({ fetchImpl, baseUrl: "http://example.test" });

    const response = await registerUser(client, {
      email: "user@example.com",
      password: "abc12345",
    });

    expect(response.ok).toBe(false);
    if (response.ok) {
      throw new Error("expected error response");
    }
    expect(response.status).toBe(409);
    expect(response.error.code).toBe("email_already_exists");
    expect(response.error.message).toContain("email already exists");
  });

  it("passes FormData through without forcing JSON headers", async () => {
    const fetchImpl: typeof fetch = vi.fn(async (_input, init?: RequestInit) => {
      expect(init?.method).toBe("POST");
      expect(init?.body).toBeInstanceOf(FormData);
      const headers = new Headers(init?.headers);
      expect(headers.has("content-type")).toBe(false);
      return new Response(JSON.stringify({ template: { id: "template-1" } }), {
        status: 201,
        headers: { "content-type": "application/json" },
      });
    });
    const client = createApiClient({ fetchImpl, baseUrl: "http://example.test" });
    const formData = new FormData();
    formData.set("name", "Support Agent");
    formData.set("soulContent", "# Soul");

    const response = await createAdminTemplate(client, formData);

    expect(response.ok).toBe(true);
    if (!response.ok) {
      throw new Error("expected success response");
    }
    expect(response.status).toBe(201);
  });

  it("archives admin templates via DELETE", async () => {
    const fetchImpl: typeof fetch = vi.fn(async (input, init?: RequestInit) => {
      const url = typeof input === "string" ? input : String(input);
      expect(url).toBe("http://example.test/api/admin/templates/template-1");
      expect(init?.method).toBe("DELETE");
      return new Response(null, { status: 204 });
    });
    const client = createApiClient({ fetchImpl, baseUrl: "http://example.test" });

    const response = await archiveAdminTemplate(client, "template-1");

    expect(response.ok).toBe(true);
    if (!response.ok) {
      throw new Error("expected success response");
    }
    expect(response.status).toBe(204);
  });
});
