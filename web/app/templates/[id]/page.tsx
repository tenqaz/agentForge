"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Button from "@/components/ui/button";
import { Card, CardDescription, CardTitle } from "@/components/ui/card";
import Input from "@/components/ui/input";
import StatusChip from "@/components/ui/status-chip";
import {
  apiErrorMessage,
  createAgent,
  getPublishedTemplate,
  type Template,
} from "@/lib/api";

export default function TemplateDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [template, setTemplate] = useState<Template | null>(null);
  const [agentName, setAgentName] = useState("");
  const [pending, setPending] = useState(false);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");

  useEffect(() => {
    if (sessionLoading) {
      return;
    }
    if (!user) {
      router.replace("/login");
      return;
    }

    let active = true;
    void (async () => {
      const response = await getPublishedTemplate(apiClient, id);
      if (!active) {
        return;
      }
      if (!response.ok) {
        setErrorStatus(response.status);
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      setTemplate(response.data.template);
      setAgentName(`${response.data.template.name} Agent`);
    })();

    return () => {
      active = false;
    };
  }, [apiClient, id, sessionLoading, router, user]);

  async function handleCreateAgent() {
    if (!template) {
      return;
    }
    setPending(true);
    setError("");
    const response = await createAgent(apiClient, template.id, agentName.trim());
    setPending(false);
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    router.push(`/agents/${response.data.agent.id}`);
  }

  return (
    <section className="grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
          Template 详情
        </p>
        <CardTitle as="h1" className="mt-2 text-3xl">
          {template?.name ?? "加载中..."}
        </CardTitle>
        <p className="mt-3 text-sm leading-7 text-[color:var(--color-fg-muted)]">
          {template?.description || "未提供描述。"}
        </p>
        {template ? (
          <div className="mt-5 flex items-center gap-2">
            <StatusChip kind="template" value={template.status} />
            <span className="font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
              v{template.version}
            </span>
          </div>
        ) : null}
      </Card>

      <Card>
        <CardTitle as="h2" className="text-xl">
          创建 Agent
        </CardTitle>
        <CardDescription>
          基于该 Template 创建一个新的 Agent，后端会自动排队配置运行时。
        </CardDescription>

        <label className="mt-5 grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
          Agent 名称
          <Input
            value={agentName}
            onChange={(event) => setAgentName(event.target.value)}
            placeholder="给 Agent 起个名字"
          />
        </label>

        {error ? (
          <div className="mt-4">
            <ApiErrorState message={error} status={errorStatus} />
          </div>
        ) : null}

        <Button
          variant="primary"
          fullWidth
          className="mt-5"
          disabled={pending || !template || !agentName.trim()}
          loading={pending}
          onClick={() => void handleCreateAgent()}
        >
          {pending ? "创建中..." : "创建 Agent"}
        </Button>
      </Card>
    </section>
  );
}
