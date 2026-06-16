import type {
  AgentResponse,
  AgentsResponse,
  ApiClientConfig,
  ApiErrorBody,
  ApiJson,
  ApiMethod,
  ApiRequestOptions,
  ApiResponse,
  ChannelResponse,
  ContentResponse,
  PairingSessionResponse,
  PairingSessionsResponse,
  RuntimeJobResponse,
  RuntimeJobsResponse,
  RuntimeResponse,
  SkillResponse,
  SkillsResponse,
  TemplateResponse,
  TemplatesResponse,
  UserResponse,
} from "./types";

export class ApiClientError extends Error {
  status: number;
  code?: string;
  details?: unknown;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.name = "ApiClientError";
    this.status = status;
    this.code = body.code;
    this.details = body.details;
  }
}

function toQueryString(
  params: ApiRequestOptions["params"],
): string | undefined {
  if (!params) return undefined;

  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null) continue;
    searchParams.set(key, String(value));
  }

  const query = searchParams.toString();
  return query.length > 0 ? `?${query}` : undefined;
}

function isJsonBody(body: unknown): body is ApiJson {
  return (
    !(typeof FormData !== "undefined" && body instanceof FormData) &&
    (
      body === null ||
      typeof body === "string" ||
      typeof body === "number" ||
      typeof body === "boolean" ||
      Array.isArray(body) ||
      (typeof body === "object" && body !== null)
    )
  );
}

function mergeHeaders(...sources: Array<HeadersInit | undefined>): Headers {
  const headers = new Headers();
  for (const source of sources) {
    if (!source) continue;
    new Headers(source).forEach((value, key) => headers.set(key, value));
  }
  return headers;
}

function normalizeBaseUrl(baseUrl?: string): string {
  if (!baseUrl) return "";
  return baseUrl.endsWith("/") ? baseUrl.slice(0, -1) : baseUrl;
}

async function parseErrorBody(response: Response): Promise<ApiErrorBody> {
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    try {
      const body = (await response.json()) as Partial<ApiErrorBody> & {
        error?: string;
      };
      return {
        message:
          body.message ??
          (typeof body.error === "string"
            ? body.error.replaceAll("_", " ")
            : response.statusText || "Request failed"),
        code: body.code ?? body.error,
        details: body.details,
      };
    } catch {
      // fall through to text handling
    }
  }

  const text = await response.text();
  return {
    message: text || response.statusText || "Request failed",
  };
}

export function createApiClient(config: ApiClientConfig = {}) {
  const fetchImpl = config.fetchImpl ?? fetch;
  const baseUrl = normalizeBaseUrl(config.baseUrl);
  const defaultHeaders = new Headers(config.defaultHeaders);

  async function request<TResponse, TBody = unknown>(
    path: string,
    options: ApiRequestOptions<TBody> = {},
  ): Promise<ApiResponse<TResponse>> {
    const url = `${baseUrl}${path}${toQueryString(options.params) ?? ""}`;
    const method: ApiMethod = options.method ?? "GET";
    const headers = mergeHeaders(defaultHeaders, options.headers);
    const init: RequestInit = {
      method,
      headers,
      signal: options.signal,
    };

    if (options.body !== undefined && method !== "GET") {
      if (isJsonBody(options.body)) {
        if (!headers.has("content-type")) {
          headers.set("content-type", "application/json");
        }
        init.body = JSON.stringify(options.body);
      } else {
        init.body = options.body as BodyInit;
      }
    }

    const response = await fetchImpl(url, init);
    if (!response.ok) {
      return {
        ok: false,
        status: response.status,
        headers: response.headers,
        error: await parseErrorBody(response),
      };
    }

    if (response.status === 204) {
      return {
        ok: true,
        status: response.status,
        headers: response.headers,
        data: undefined as TResponse,
      };
    }

    const contentType = response.headers.get("content-type") ?? "";
    if (contentType.includes("application/json")) {
      return {
        ok: true,
        status: response.status,
        headers: response.headers,
        data: (await response.json()) as TResponse,
      };
    }

    return {
      ok: true,
      status: response.status,
      headers: response.headers,
      data: (await response.text()) as TResponse,
    };
  }

  return {
    request,
    get<TResponse>(path: string, options?: Omit<ApiRequestOptions, "method" | "body">) {
      return request<TResponse>(path, { ...options, method: "GET" });
    },
    post<TResponse, TBody = unknown>(path: string, body?: TBody, options?: Omit<ApiRequestOptions<TBody>, "method" | "body">) {
      return request<TResponse, TBody>(path, { ...options, method: "POST", body });
    },
    put<TResponse, TBody = unknown>(path: string, body?: TBody, options?: Omit<ApiRequestOptions<TBody>, "method" | "body">) {
      return request<TResponse, TBody>(path, { ...options, method: "PUT", body });
    },
    patch<TResponse, TBody = unknown>(path: string, body?: TBody, options?: Omit<ApiRequestOptions<TBody>, "method" | "body">) {
      return request<TResponse, TBody>(path, { ...options, method: "PATCH", body });
    },
    delete<TResponse>(path: string, options?: Omit<ApiRequestOptions, "method" | "body">) {
      return request<TResponse>(path, { ...options, method: "DELETE" });
    },
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;

export async function getSession(client: ApiClient) {
  return client.get<UserResponse>("/api/session");
}

export async function registerUser(
  client: ApiClient,
  payload: { email: string; password: string },
) {
  return client.post<UserResponse, { email: string; password: string }>(
    "/api/users",
    payload,
  );
}

export async function listPublishedTemplates(client: ApiClient) {
  return client.get<TemplatesResponse>("/api/templates");
}

export async function getPublishedTemplate(client: ApiClient, templateId: string) {
  return client.get<TemplateResponse>(`/api/templates/${templateId}`);
}

export async function listAdminTemplates(client: ApiClient) {
  return client.get<TemplatesResponse>("/api/admin/templates");
}

export async function getAdminTemplate(client: ApiClient, templateId: string) {
  return client.get<TemplateResponse>(`/api/admin/templates/${templateId}`);
}

export async function createAdminTemplate(
  client: ApiClient,
  payload: FormData,
) {
  return client.post<TemplateResponse, FormData>(
    "/api/admin/templates",
    payload,
  );
}

export async function archiveAdminTemplate(client: ApiClient, templateId: string) {
  return client.delete<void>(`/api/admin/templates/${templateId}`);
}

export async function updateAdminTemplate(
  client: ApiClient,
  templateId: string,
  name: string,
  description: string,
) {
  return client.put<TemplateResponse, { name: string; description: string }>(
    `/api/admin/templates/${templateId}`,
    { name, description },
  );
}

export async function getTemplateSoul(client: ApiClient, templateId: string) {
  return client.get<ContentResponse>(`/api/admin/templates/${templateId}/soul`);
}

export async function saveTemplateSoul(
  client: ApiClient,
  templateId: string,
  content: string,
) {
  return client.put<TemplateResponse, { content: string }>(
    `/api/admin/templates/${templateId}/soul`,
    { content },
  );
}

export async function getTemplateUser(client: ApiClient, templateId: string) {
  return client.get<ContentResponse>(`/api/admin/templates/${templateId}/user`);
}

export async function saveTemplateUser(
  client: ApiClient,
  templateId: string,
  content: string,
) {
  return client.put<TemplateResponse, { content: string }>(
    `/api/admin/templates/${templateId}/user`,
    { content },
  );
}

export async function listTemplateSkills(client: ApiClient, templateId: string) {
  return client.get<SkillsResponse>(`/api/admin/templates/${templateId}/skills`);
}

export async function addTemplateSkill(
  client: ApiClient,
  templateId: string,
  skillName: string,
  skillMD: string,
) {
  return client.post<SkillResponse, { skillName: string; skillMD: string }>(
    `/api/admin/templates/${templateId}/skills`,
    { skillName, skillMD },
  );
}

export async function deleteTemplateSkill(
  client: ApiClient,
  templateId: string,
  skillId: string,
) {
  return client.delete<TemplateResponse>(
    `/api/admin/templates/${templateId}/skills/${skillId}`,
  );
}

export async function publishTemplate(client: ApiClient, templateId: string) {
  return client.put<TemplateResponse>(
    `/api/admin/templates/${templateId}/publication`,
    {},
  );
}

export async function unpublishTemplate(client: ApiClient, templateId: string) {
  return client.delete<TemplateResponse>(
    `/api/admin/templates/${templateId}/publication`,
  );
}

export async function listAgents(client: ApiClient) {
  return client.get<AgentsResponse>("/api/agents");
}

export async function createAgent(
  client: ApiClient,
  templateId: string,
  name: string,
) {
  return client.post<AgentResponse, { templateId: string; name: string }>(
    "/api/agents",
    { templateId, name },
  );
}

export async function getAgent(client: ApiClient, agentId: string) {
  return client.get<AgentResponse>(`/api/agents/${agentId}`);
}

export async function getRuntime(client: ApiClient, agentId: string) {
  return client.get<RuntimeResponse>(`/api/agents/${agentId}/runtime`);
}

export async function listRuntimeJobs(client: ApiClient, agentId: string) {
  return client.get<RuntimeJobsResponse>(`/api/agents/${agentId}/runtime-jobs`);
}

export async function restartRuntime(client: ApiClient, agentId: string) {
  return client.post<RuntimeJobResponse, { type: "restart_runtime" }>(
    `/api/agents/${agentId}/runtime-jobs`,
    { type: "restart_runtime" },
  );
}

export async function getWeixinChannel(client: ApiClient, agentId: string) {
  return client.get<ChannelResponse>(`/api/agents/${agentId}/channels/weixin`);
}

export async function ensureWeixinChannel(client: ApiClient, agentId: string) {
  return client.put<ChannelResponse>(`/api/agents/${agentId}/channels/weixin`, {});
}

export async function listWeixinPairingSessions(
  client: ApiClient,
  agentId: string,
) {
  return client.get<PairingSessionsResponse>(
    `/api/agents/${agentId}/channels/weixin/pairing-sessions`,
  );
}

export async function createWeixinPairingSession(
  client: ApiClient,
  agentId: string,
) {
  return client.post<PairingSessionResponse>(
    `/api/agents/${agentId}/channels/weixin/pairing-sessions`,
    {},
  );
}

export async function getWeixinPairingSession(
  client: ApiClient,
  agentId: string,
  sessionId: string,
) {
  return client.get<PairingSessionResponse>(
    `/api/agents/${agentId}/channels/weixin/pairing-sessions/${sessionId}`,
  );
}

export function apiErrorMessage(
  code?: string,
  fallback = "Unexpected error",
) {
  switch (code) {
    case "invalid_credentials":
      return "Email or password is incorrect.";
    case "unauthorized":
      return "Sign in is required.";
    case "forbidden":
      return "This page requires an admin account.";
    case "agent_not_running":
      return "The agent is not running yet.";
    case "runtime_unavailable":
      return "The runtime is not available yet.";
    case "conflict":
      return "This action conflicts with the current resource state.";
    case "invalid_request":
      return "The submitted data is invalid.";
    case "invalid_email":
      return "Enter a valid email address.";
    case "invalid_password":
      return "Password must be at least 8 characters and include a letter and a number.";
    case "invalid_template":
      return "The template is incomplete and cannot be published yet.";
    case "email_already_exists":
      return "An account with this email already exists.";
    case "email_conflict":
      return "This email cannot be used right now. Please contact support.";
    case "not_found":
      return "The requested resource could not be found.";
    case "internal_error":
      return "The server returned an internal error.";
    default:
      return code ? code.replaceAll("_", " ") : fallback;
  }
}
