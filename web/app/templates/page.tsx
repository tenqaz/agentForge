"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Inbox, LayoutTemplate } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import EmptyState from "@/components/ui/empty-state";
import Spinner from "@/components/ui/spinner";
import { apiErrorMessage, listPublishedTemplates, type Template } from "@/lib/api";

export default function TemplatesPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [fetching, setFetching] = useState(true);

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
      if (!active) {
        return;
      }
      setFetching(false);
      if (!response.ok) {
        setErrorStatus(response.status);
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      setTemplates(response.data.templates);
    })();

    return () => {
      active = false;
    };
  }, [apiClient, sessionLoading, router, user]);

  return (
    <>
      <div className="page-head">
        <div>
          <span className="meta">模板浏览</span>
          <h1>选择一个模板启动 Agent</h1>
          <p className="lead">只有已发布的模板出现在这里。创建 Agent 后，后端会立即排队执行运行时配置。</p>
        </div>
      </div>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="row" style={{ color: "var(--muted)", fontSize: 13 }}>
          <Spinner size="sm" />
          <span>加载中…</span>
        </div>
      ) : templates.length === 0 && !error ? (
        <EmptyState
          icon={<Inbox size={24} strokeWidth={1.5} />}
          title="暂无已发布的模板"
          description="管理员发布之后，可用的模板会显示在这里。"
        />
      ) : (
        <div className="grid-3">
          {templates.map((template) => (
            <Link key={template.id} href={`/templates/${template.id}`} className="card">
              <div className="row" style={{ gap: 12, alignItems: "flex-start" }}>
                <span className="avatar" style={{ width: 36, height: 36, borderRadius: 10 }} aria-hidden="true">
                  <LayoutTemplate size={18} strokeWidth={1.75} />
                </span>
                <div style={{ minWidth: 0 }}>
                  <h3 style={{ fontSize: 15, fontWeight: 600 }}>{template.name}</h3>
                </div>
              </div>
              <p className="muted" style={{ fontSize: 13, lineHeight: 1.55, marginTop: 10 }}>
                {template.description || "暂无描述。"}
              </p>
              <div className="row" style={{ gap: 8, marginTop: 14, paddingTop: 12, borderTop: "1px dashed var(--border)" }}>
                <span className="tag tag-mono">v{template.version}</span>
                <span className="tag tag-mono">已发布</span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </>
  );
}
