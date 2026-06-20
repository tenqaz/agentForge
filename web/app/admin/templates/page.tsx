"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Inbox, Plus, Trash2 } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Button from "@/components/ui/button";
import EmptyState from "@/components/ui/empty-state";
import Modal from "@/components/ui/modal";
import Spinner from "@/components/ui/spinner";
import StatusChip from "@/components/ui/status-chip";
import Tabs from "@/components/ui/tabs";
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
  const [tab, setTab] = useState<string>("published");

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

  const drafts = templates.filter((t) => t.status === "draft");
  const published = templates.filter((t) => t.status === "published");
  const archived = templates.filter((t) => t.status === "archived");
  const visible = tab === "published" ? published : tab === "drafts" ? drafts : archived;
  const confirmingTemplate = templates.find((t) => t.id === confirmingTemplateId) ?? null;

  return (
    <>
      <div className="page-head">
        <div>
          <span className="meta">模板管理</span>
          <h1>模板管理</h1>
          <p className="lead">起草、发布、归档模板版本。已发布的模板对所有用户可见。</p>
        </div>
        <Link href="/admin/templates/new">
          <Button variant="primary" leftIcon={<Plus size={16} strokeWidth={1.75} />}>
            新建草稿
          </Button>
        </Link>
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
          title="还没有模板"
          description="点击右上角“新建草稿”创建你的第一个模板。"
        />
      ) : (
        <>
          <Tabs
            items={[
              { key: "drafts", label: "草稿", count: drafts.length },
              { key: "published", label: "已发布", count: published.length },
              { key: "archived", label: "归档", count: archived.length },
            ]}
            value={tab}
            onChange={setTab}
          />

          {visible.length === 0 ? (
            <EmptyState
              icon={<Inbox size={24} strokeWidth={1.5} />}
              title="该分类下没有模板"
              description="切换到其它分类查看。"
            />
          ) : (
            <section className="section-card">
              <div className="section-card-body no-pad">
                <table className="table">
                  <thead>
                    <tr>
                      <th>名称</th>
                      <th>状态</th>
                      <th>版本</th>
                      <th>更新</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {visible.map((template) => (
                      <tr key={template.id}>
                        <td>
                          <Link href={`/admin/templates/${template.id}`} style={{ display: "inline-flex", alignItems: "center", gap: 10 }}>
                            <span className="avatar" style={{ width: 28, height: 28, fontSize: 12 }} aria-hidden="true">
                              {template.name.slice(0, 1)}
                            </span>
                            <span>
                              <span style={{ fontWeight: 500 }}>{template.name}</span>
                              <span className="meta" style={{ display: "block" }}>
                                {template.description || "暂无描述"}
                              </span>
                            </span>
                          </Link>
                        </td>
                        <td>
                          <StatusChip kind="template" value={template.status} />
                        </td>
                        <td>
                          <span className="tag tag-mono">v{template.version}</span>
                        </td>
                        <td className="meta num-col">{template.updatedAt.slice(0, 10)}</td>
                        <td className="row-actions">
                          <Link href={`/admin/templates/${template.id}`}>
                            <Button variant="secondary" size="sm">
                              编辑
                            </Button>
                          </Link>
                          {template.status !== "archived" ? (
                            <Button
                              variant="ghost"
                              size="sm"
                              leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
                              disabled={pendingTemplateId === template.id}
                              onClick={() => setConfirmingTemplateId(template.id)}
                              style={{ color: "var(--danger)" }}
                            >
                              归档
                            </Button>
                          ) : null}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}
        </>
      )}

      <Modal
        open={confirmingTemplateId !== ""}
        onClose={() => setConfirmingTemplateId("")}
        title={`归档「${confirmingTemplate?.name ?? ""}」？`}
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setConfirmingTemplateId("")}>
              取消
            </Button>
            <Button
              variant="danger"
              size="sm"
              disabled={pendingTemplateId === confirmingTemplateId}
              loading={pendingTemplateId === confirmingTemplateId}
              onClick={() => confirmingTemplateId && void handleArchive(confirmingTemplateId)}
            >
              {pendingTemplateId === confirmingTemplateId ? "归档中…" : "确认归档"}
            </Button>
          </>
        }
      >
        <p>归档后该模板不再对用户可见，但已基于它创建的 Agent 不受影响。</p>
      </Modal>
    </>
  );
}
