import { describe, expect, it, vi } from "vitest";

import { createApiClient } from "@/lib/api";

describe("createApiClient", () => {
  it("maps backend error payloads that only expose error codes", async () => {
    const fetchImpl = vi.fn(async () =>
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
    const fetchImpl = vi.fn(async (_url: string, init?: RequestInit) => {
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
});
