"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ChevronRight, Inbox, LayoutTemplate, Plus, Trash2 } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Button from "@/components/ui/button";
import EmptyState from "@/components/ui/empty-state";
import Spinner from "@/components/ui/spinner";
import StatusChip from "@/components/ui/status-chip";
import {
  apiErrorMessage,
  archiveAdminTemplate,
  listAdminTemplates,
  type Template,
} from "@/lib/api";

export default function AdminTemplatesPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [confirmingTemplateId, setConfirmingTemplateId] = useState("");
  const [pendingTemplateId, setPendingTemplateId] = useState("");
  const [fetching, setFetching] = useState(true);

  useEffect(() => {
    if (sessionLoading) {
      return;
    }
    if (!user) {
      router.replace("/login");
      return;
    }
    if (user.role !== "admin") {
      router.replace("/templates");
      return;
    }

    let active = true;
    void (async () => {
      const response = await listAdminTemplates(apiClient);
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

  async function handleArchive(templateId: string) {
    setPendingTemplateId(templateId);
    setError("");
    const response = await archiveAdminTemplate(apiClient, templateId);
    setPendingTemplateId("");
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    setConfirmingTemplateId("");
    setTemplates((current) => current.filter((template) => template.id !== templateId));
  }

  return (
    <section className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-col gap-1.5">
          <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
            管理 Templates
          </p>
          <h1 className="text-2xl font-semibold tracking-tight text-[color:var(--color-fg)] sm:text-3xl">
            起草、发布、克隆 Template 版本
          </h1>
        </div>
        <Link href="/admin/templates/new">
          <Button variant="primary" leftIcon={<Plus size={16} strokeWidth={1.75} />}>
            新建草稿
          </Button>
        </Link>
      </header>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="flex items-center gap-2 px-1 text-sm text-[color:var(--color-fg-muted)]">
          <Spinner size="sm" />
          <span>加载中...</span>
        </div>
      ) : templates.length === 0 && !error ? (
        <EmptyState
          icon={<Inbox size={24} strokeWidth={1.5} />}
          title="还没有 Template"
          description="点击右上角“新建草稿”创建你的第一个 Template。"
        />
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {templates.map((template) => (
            <div
              key={template.id}
              className="rounded-[var(--radius-xl)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)] p-5"
            >
              <Link href={`/admin/templates/${template.id}`} className="group block">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-3 min-w-0">
                    <span
                      className="grid size-9 shrink-0 place-items-center rounded-[var(--radius-md)] bg-[color:var(--color-bg-hover)] text-[color:var(--color-fg-muted)] group-hover:text-[color:var(--color-accent)]"
                      aria-hidden="true"
                    >
                      <LayoutTemplate size={18} strokeWidth={1.75} />
                    </span>
                    <div className="min-w-0">
                      <h2 className="truncate text-base font-semibold text-[color:var(--color-fg)]">
                        {template.name}
                      </h2>
                      <p className="mt-1 line-clamp-2 text-sm leading-6 text-[color:var(--color-fg-muted)]">
                        {template.description || "暂无描述。"}
                      </p>
                    </div>
                  </div>
                  <ChevronRight
                    size={16}
                    strokeWidth={1.75}
                    className="mt-1 shrink-0 text-[color:var(--color-fg-subtle)] transition group-hover:translate-x-0.5 group-hover:text-[color:var(--color-fg-muted)]"
                    aria-hidden="true"
                  />
                </div>
                <div className="mt-4 flex items-center gap-2">
                  <StatusChip kind="template" value={template.status} />
                  <span className="font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
                    v{template.version}
                  </span>
                </div>
              </Link>

              <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-[color:var(--color-border-subtle)] pt-4">
                {confirmingTemplateId === template.id ? (
                  <>
                    <Button
                      variant="danger"
                      size="sm"
                      disabled={pendingTemplateId === template.id}
                      loading={pendingTemplateId === template.id}
                      onClick={() => void handleArchive(template.id)}
                    >
                      {pendingTemplateId === template.id ? "删除中..." : "确认删除"}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={pendingTemplateId === template.id}
                      onClick={() => setConfirmingTemplateId("")}
                    >
                      取消
                    </Button>
                  </>
                ) : (
                  <Button
                    variant="ghost"
                    size="sm"
                    leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
                    disabled={pendingTemplateId === template.id}
                    onClick={() => setConfirmingTemplateId(template.id)}
                    className="text-[color:var(--color-danger)] hover:bg-[color:var(--color-danger-soft)] hover:text-[color:var(--color-danger)]"
                  >
                    删除 Template
                  </Button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
