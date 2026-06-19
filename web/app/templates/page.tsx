"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ChevronRight, Inbox, LayoutTemplate } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import EmptyState from "@/components/ui/empty-state";
import StatusChip from "@/components/ui/status-chip";
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
    <section className="flex flex-col gap-6">
      <header className="flex flex-col gap-1.5">
        <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
          已发布的 Templates
        </p>
        <h1 className="text-2xl font-semibold tracking-tight text-[color:var(--color-fg)] sm:text-3xl">
          选择一个 Template 启动 Agent
        </h1>
        <p className="max-w-2xl text-sm leading-6 text-[color:var(--color-fg-muted)]">
          只有已发布的 Template 出现在这里。创建 Agent 后，后端会立即排队执行运行时配置。
        </p>
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
          title="暂无已发布的 Template"
          description="管理员发布之后，可用的 Template 会显示在这里。"
        />
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {templates.map((template) => (
            <Link
              key={template.id}
              href={`/templates/${template.id}`}
              className="group block rounded-[var(--radius-xl)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)] p-5 transition hover:border-[color:var(--color-border-default)] hover:bg-[color:var(--color-bg-hover)]"
            >
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
          ))}
        </div>
      )}
    </section>
  );
}
