export type ApiPrimitive = string | number | boolean | null;
export type ApiJson = ApiPrimitive | ApiJson[] | { [key: string]: ApiJson };

/**
 * 每个 template 最多可拥有的 skill 数量。
 * 必须与后端 templates.MaxSkillsPerTemplate 保持一致。
 */
export const MAX_SKILLS_PER_TEMPLATE = 20;

export type ApiMethod =
  | "DELETE"
  | "GET"
  | "PATCH"
  | "POST"
  | "PUT";

export type ApiErrorBody = {
  message: string;
  code?: string;
  details?: unknown;
};

export type ApiResponse<T> =
  | {
      ok: true;
      data: T;
      status: number;
      headers: Headers;
    }
  | {
      ok: false;
      error: ApiErrorBody;
      status: number;
      headers: Headers;
    };

export type ApiRequestOptions<TBody = unknown> = {
  method?: ApiMethod;
  body?: TBody;
  headers?: HeadersInit;
  signal?: AbortSignal;
  params?: Record<string, string | number | boolean | undefined | null>;
};

export type ApiClientConfig = {
  baseUrl?: string;
  fetchImpl?: typeof fetch;
  defaultHeaders?: HeadersInit;
};

export type UserRole = "admin" | "user";

export type TemplateStatus = "draft" | "published" | "archived";
export type AgentStatus =
  | "creating"
  | "provisioning"
  | "starting"
  | "running"
  | "stopped"
  | "error";
export type RuntimeJobType =
  | "provision_agent"
  | "start_runtime"
  | "stop_runtime"
  | "restart_runtime";
export type JobStatus =
  | "queued"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled";
export type ChannelStatus =
  | "not_configured"
  | "qr_pending"
  | "connected"
  | "error"
  | "disconnected";
export type PairingStatus = "pending" | "connected" | "expired" | "failed";

export type User = {
  id: string;
  email: string;
  role: UserRole;
};

export type Template = {
  id: string;
  name: string;
  description: string;
  status: TemplateStatus;
  version: number;
  createdAt: string;
  updatedAt: string;
  publishedAt?: string | null;
};

export type Skill = {
  id: string;
  templateId: string;
  skillName: string;
  checksum: string;
  createdAt: string;
};

export type Agent = {
  id: string;
  ownerUserId: string;
  templateId: string;
  templateVersion: number;
  name: string;
  status: AgentStatus;
  runtimeId: string;
  lastErrorCode: string;
  lastErrorMessage: string;
  createdAt: string;
  updatedAt: string;
};

export type AgentRuntime = {
  agentId: string;
  runtimeId: string;
  status: AgentStatus;
  lastErrorCode: string;
  lastErrorMessage: string;
  updatedAt: string;
};

export type RuntimeJob = {
  id: string;
  agentId: string;
  type: RuntimeJobType;
  status: JobStatus;
  priority: number;
  attemptCount: number;
  maxAttempts: number;
  lockedUntil?: string | null;
  lastErrorCode: string;
  lastErrorMessage: string;
  createdAt: string;
  updatedAt: string;
  startedAt?: string | null;
  finishedAt?: string | null;
};

export type Channel = {
  id?: string;
  agentId?: string;
  channelType: "weixin";
  status: ChannelStatus;
  externalAccountId?: string;
  lastErrorCode?: string;
  lastErrorMessage?: string;
};

export type PairingSession = {
  id: string;
  status: PairingStatus;
  qrPayload?: string;
  // qrPayloadUrl is the scannable liteapp URL (plain text, e.g.
  // https://liteapp.weixin.qq.com/q/...). The component must encode it
  // into a QR image client-side; it is NOT image data.
  qrPayloadUrl?: string;
  expiresAt: string;
};

export type UserResponse = {
  user: User;
};

export type EmailCodeResponse = {
  ok: boolean;
};

export type TemplatesResponse = {
  templates: Template[];
};

export type TemplateResponse = {
  template: Template;
};

export type ContentResponse = {
  content: string;
};

export type SkillsResponse = {
  skills: Skill[];
};

export type SkillResponse = {
  skill: Skill;
  content?: string;
};

export type AgentsResponse = {
  agents: Agent[];
};

export type AgentResponse = {
  agent: Agent;
};

export type RuntimeResponse = {
  runtime: AgentRuntime;
};

export type RuntimeJobsResponse = {
  jobs: RuntimeJob[];
};

export type RuntimeJobResponse = {
  job: RuntimeJob;
};

export type ChannelResponse = {
  channel: Channel;
};

export type PairingSessionResponse = {
  session: PairingSession;
};

export type PairingSessionsResponse = {
  sessions: PairingSession[];
};
