"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Breadcrumbs from "@/components/ui/breadcrumbs";
import Button from "@/components/ui/button";
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
    router.push(`/agents/${response.data.agent.id}/provision`);
  }

  return (
    <>
      <Breadcrumbs
        items={[
          { label: "模板浏览", href: "/templates" },
          { label: template?.name ?? "模板" },
        ]}
      />

      <div className="grid-2-1">
        <section className="section-card">
          <div className="section-card-head">
            <div>
              <span className="meta">模板详情</span>
              <h1 style={{ fontSize: 24, fontWeight: 600, marginTop: 4 }}>
                {template?.name ?? "加载中…"}
              </h1>
            </div>
            {template ? (
              <div className="row" style={{ gap: 8 }}>
                <StatusChip kind="template" value={template.status} />
                <span className="tag tag-mono">v{template.version}</span>
              </div>
            ) : null}
          </div>
          <div className="section-card-body">
            <p className="muted" style={{ fontSize: 14, lineHeight: 1.7 }}>
              {template?.description || "未提供描述。"}
            </p>
          </div>
        </section>

        <section className="section-card">
          <div className="section-card-head">
            <h3 style={{ fontSize: 15, fontWeight: 600 }}>创建 Agent</h3>
          </div>
          <div className="section-card-body">
            <p className="muted" style={{ fontSize: 13, marginBottom: 16 }}>
              基于该模板创建一个新的 Agent，后端会自动排队配置运行时。
            </p>
            <div className="form-stack">
              <div className="field">
                <label className="field-label" htmlFor="agent-name-input">Agent 名称</label>
                <Input
                  id="agent-name-input"
                  value={agentName}
                  onChange={(event) => setAgentName(event.target.value)}
                  placeholder="给 Agent 起个名字"
                />
              </div>

              {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

              <Button
                variant="primary"
                fullWidth
                disabled={pending || !template || !agentName.trim()}
                loading={pending}
                onClick={() => void handleCreateAgent()}
              >
                {pending ? "创建中…" : "创建 Agent"}
              </Button>
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
