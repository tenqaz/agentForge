"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ArrowRight, Check } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Breadcrumbs from "@/components/ui/breadcrumbs";
import Button from "@/components/ui/button";
import EmptyState from "@/components/ui/empty-state";
import Input from "@/components/ui/input";
import Spinner from "@/components/ui/spinner";
import {
  apiErrorMessage,
  createAgent,
  listPublishedTemplates,
  type Template,
} from "@/lib/api";

export default function NewAgentPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [agentName, setAgentName] = useState("");
  const [fetching, setFetching] = useState(true);
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
      const response = await listPublishedTemplates(apiClient);
      if (!active) return;
      setFetching(false);
      if (!response.ok) {
        setErrorStatus(response.status);
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      setTemplates(response.data.templates);
      const first = response.data.templates[0];
      if (first) {
        setSelectedId(first.id);
        setAgentName(`${first.name} Agent`);
      }
    })();
    return () => {
      active = false;
    };
  }, [apiClient, sessionLoading, router, user]);

  function selectTemplate(template: Template) {
    setSelectedId(template.id);
    if (!agentName || agentName === `${templates.find((t) => t.id === selectedId)?.name} Agent`) {
      setAgentName(`${template.name} Agent`);
    }
  }

  async function handleSubmit() {
    if (!selectedId || !agentName.trim()) return;
    setPending(true);
    setError("");
    const response = await createAgent(apiClient, selectedId, agentName.trim());
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
          { label: "我的 Agents", href: "/agents" },
          { label: "新建 Agent" },
        ]}
      />

      <div className="page-head">
        <div>
          <h1>新建 Agent</h1>
          <p className="lead">从公开模板里挑一个人格，给它取个名字。提交后平台会异步准备数据目录与运行时。</p>
        </div>
      </div>

      <div className="wizard-steps">
        <span className="ws is-active">
          <span className="ws-num">1</span>选模板 + 命名
        </span>
        <span className="ws">
          <span className="ws-num">2</span>异步供应
        </span>
        <span className="ws">
          <span className="ws-num">3</span>微信扫码
        </span>
      </div>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="row" style={{ color: "var(--muted)", fontSize: 13 }}>
          <Spinner size="sm" />
          <span>加载模板中…</span>
        </div>
      ) : templates.length === 0 ? (
        <EmptyState
          title="暂无可用模板"
          description="管理员发布模板后，你可以在这里创建 Agent。"
          action={
            <Link href="/templates">
              <Button variant="primary">浏览模板</Button>
            </Link>
          }
        />
      ) : (
        <>
          <section className="section-card">
            <div className="section-card-head">
              <h3 style={{ fontSize: 15, fontWeight: 600 }}>1 · 选择模板</h3>
              <Link href="/templates" className="meta">
                浏览全部模板 →
              </Link>
            </div>
            <div className="section-card-body">
              <div className="tmpl-grid">
                {templates.map((template) => (
                  <div
                    key={template.id}
                    className={`tmpl-card${template.id === selectedId ? " is-selected" : ""}`}
                    onClick={() => selectTemplate(template)}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        selectTemplate(template);
                      }
                    }}
                  >
                    <div className="t-head">
                      <span className="t-mark">{template.name.slice(0, 1)}</span>
                      <span className="meta">v{template.version} · 已发布</span>
                    </div>
                    <h4>{template.name}</h4>
                    <p>{template.description || "暂无描述。"}</p>
                    <div className="t-foot">
                      <span className="tag tag-mono">已发布</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className="section-card">
            <div className="section-card-head">
              <h3 style={{ fontSize: 15, fontWeight: 600 }}>2 · 命名</h3>
            </div>
            <div className="section-card-body">
              <div className="form-stack" style={{ maxWidth: 520 }}>
                <div className="field">
                  <label className="field-label" htmlFor="new-agent-name">Agent 名称</label>
                  <Input
                    id="new-agent-name"
                    value={agentName}
                    onChange={(event) => setAgentName(event.target.value)}
                    placeholder="例如：我的私人助理"
                    maxLength={32}
                  />
                  <span className="field-help">这只是显示名称，可以随时改。</span>
                </div>
              </div>
            </div>
          </section>

          <section className="section-card">
            <div className="section-card-head">
              <h3 style={{ fontSize: 15, fontWeight: 600 }}>3 · 创建后会发生什么</h3>
            </div>
            <div className="section-card-body">
              <div className="stack-sm" style={{ gap: 12 }}>
                <div className="row" style={{ gap: 12, alignItems: "flex-start" }}>
                  <span style={{ color: "var(--success)", marginTop: 4 }} aria-hidden="true">
                    <Check size={14} strokeWidth={2} />
                  </span>
                  <span style={{ fontSize: 13.5 }}>分配独立数据目录，与其他 Agent 完全隔离。</span>
                </div>
                <div className="row" style={{ gap: 12, alignItems: "flex-start" }}>
                  <span style={{ color: "var(--success)", marginTop: 4 }} aria-hidden="true">
                    <Check size={14} strokeWidth={2} />
                  </span>
                  <span style={{ fontSize: 13.5 }}>从模板复制 SOUL.md / USER.md 与所有 skills，并锁定模板版本。</span>
                </div>
                <div className="row" style={{ gap: 12, alignItems: "flex-start" }}>
                  <span style={{ color: "var(--success)", marginTop: 4 }} aria-hidden="true">
                    <Check size={14} strokeWidth={2} />
                  </span>
                  <span style={{ fontSize: 13.5 }}>启动专属运行时容器，进入 RUNNING 状态后解锁微信扫码。</span>
                </div>
              </div>
            </div>
          </section>

          <div className="form-actions">
            <Button
              variant="primary"
              size="lg"
              disabled={pending || !selectedId || !agentName.trim()}
              loading={pending}
              onClick={() => void handleSubmit()}
              rightIcon={pending ? undefined : <ArrowRight size={16} strokeWidth={1.75} />}
            >
              创建 Agent
            </Button>
            <Link href="/agents">
              <Button variant="ghost">取消</Button>
            </Link>
            <span className="meta" style={{ marginLeft: "auto" }}>
              下一步：异步供应（数十秒）
            </span>
          </div>
        </>
      )}
    </>
  );
}
