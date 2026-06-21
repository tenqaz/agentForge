"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Trash2 } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
import Input from "@/components/ui/input";
import StatusChip from "@/components/ui/status-chip";
import Textarea from "@/components/ui/textarea";
import {
  archiveAdminTemplate,
  apiErrorMessage,
  publishTemplate,
  saveTemplateSoul,
  saveTemplateUser,
  unpublishTemplate,
  updateAdminTemplate,
  type Template,
} from "@/lib/api";

export default function TemplateEditor({
  initialTemplate,
  initialSoul,
  initialUserContent,
  busySection,
  onBusySectionChange,
  onTemplateChange,
}: {
  initialTemplate: Template;
  initialSoul: string;
  initialUserContent: string;
  busySection: string;
  onBusySectionChange: (section: string) => void;
  onTemplateChange: (template: Template, soul: string, userContent: string) => void;
}) {
  const apiClient = useApiClient();
  const router = useRouter();
  const [template, setTemplate] = useState(initialTemplate);
  const [name, setName] = useState(initialTemplate.name);
  const [description, setDescription] = useState(initialTemplate.description);
  const [soul, setSoul] = useState(initialSoul);
  const [userContent, setUserContent] = useState(initialUserContent);
  const [error, setError] = useState("");
  const [pending, setPending] = useState<string>("");
  const [confirmArchive, setConfirmArchive] = useState(false);

  function applyTemplate(nextTemplate: Template, nextSoul = soul, nextUser = userContent) {
    setTemplate(nextTemplate);
    setName(nextTemplate.name);
    setDescription(nextTemplate.description);
    onTemplateChange(nextTemplate, nextSoul, nextUser);
    if (nextTemplate.id !== template.id) {
      router.replace(`/admin/templates/${nextTemplate.id}`);
    }
  }

  async function handleMetadataSave() {
    setPending("metadata");
    onBusySectionChange("metadata");
    setError("");
    const response = await updateAdminTemplate(apiClient, template.id, name, description);
    setPending("");
    onBusySectionChange("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    applyTemplate(response.data.template);
  }

  async function handleSoulSave() {
    setPending("soul");
    onBusySectionChange("soul");
    setError("");
    const response = await saveTemplateSoul(apiClient, template.id, soul);
    setPending("");
    onBusySectionChange("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    applyTemplate(response.data.template, soul);
  }

  async function handleUserSave() {
    setPending("user");
    onBusySectionChange("user");
    setError("");
    const response = await saveTemplateUser(apiClient, template.id, userContent);
    setPending("");
    onBusySectionChange("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    applyTemplate(response.data.template, soul, userContent);
  }

  async function handlePublishToggle() {
    setPending("publication");
    onBusySectionChange("publication");
    setError("");
    const response =
      template.status === "published"
        ? await unpublishTemplate(apiClient, template.id)
        : await publishTemplate(apiClient, template.id);
    setPending("");
    onBusySectionChange("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    applyTemplate(response.data.template);
  }

  async function handleArchive() {
    setPending("archive");
    onBusySectionChange("archive");
    setError("");
    const response = await archiveAdminTemplate(apiClient, template.id);
    setPending("");
    onBusySectionChange("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    router.replace("/admin/templates");
  }

  const isPublished = template.status === "published";
  const publishLabel = isPublished
    ? pending === "publication"
      ? "取消发布中…"
      : "取消发布"
    : pending === "publication"
      ? "发布中…"
      : "发布";
  const skillsBusy = busySection === "skills";

  return (
    <div className="stack">
      {/* 元数据与发布 */}
      <section className="section-card">
        <div className="section-card-head">
          <div className="row" style={{ gap: 12, alignItems: "flex-start" }}>
            <div>
              <span className="meta">Template</span>
              <h3 style={{ fontSize: 15, fontWeight: 600, marginTop: 4 }}>元数据与发布</h3>
              <div className="row" style={{ gap: 8, marginTop: 10 }}>
                <StatusChip kind="template" value={template.status} />
                <span className="tag tag-mono">v{template.version}</span>
              </div>
            </div>
          </div>
          <div className="row" style={{ gap: 8, flexWrap: "wrap", justifyContent: "flex-end" }}>
            <Button
              variant={isPublished ? "secondary" : "primary"}
              size="sm"
              disabled={pending === "publication" || pending === "archive" || skillsBusy}
              loading={pending === "publication"}
              onClick={() => void handlePublishToggle()}
            >
              {publishLabel}
            </Button>
            {confirmArchive ? (
              <>
                <Button
                  variant="danger"
                  size="sm"
                  disabled={pending === "archive" || skillsBusy}
                  loading={pending === "archive"}
                  onClick={() => void handleArchive()}
                >
                  {pending === "archive" ? "删除中…" : "确认删除"}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={pending === "archive" || skillsBusy}
                  onClick={() => setConfirmArchive(false)}
                >
                  取消
                </Button>
              </>
            ) : (
              <Button
                variant="ghost"
                size="sm"
                leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
                disabled={pending === "publication" || pending === "archive" || skillsBusy}
                onClick={() => setConfirmArchive(true)}
                style={{ color: "var(--danger)" }}
              >
                删除 Template
              </Button>
            )}
          </div>
        </div>

        <div className="section-card-body">
          <div className="form-stack">
            <div className="field">
              <label className="field-label">名称</label>
              <Input value={name} onChange={(event) => setName(event.target.value)} />
            </div>
            <div className="field">
              <label className="field-label">描述</label>
              <Textarea rows={3} value={description} onChange={(event) => setDescription(event.target.value)} />
            </div>
            <div className="form-actions">
              <Button
                variant="primary"
                size="sm"
                disabled={pending === "metadata"}
                loading={pending === "metadata"}
                onClick={() => void handleMetadataSave()}
              >
                {pending === "metadata" ? "保存中…" : "保存元数据"}
              </Button>
            </div>
          </div>
        </div>
      </section>

      {/* SOUL.md */}
      <section className="section-card">
        <div className="section-card-head">
          <h3 style={{ fontSize: 15, fontWeight: 600 }}>SOUL.md · 人格</h3>
          <Button
            variant="primary"
            size="sm"
            disabled={pending === "soul"}
            loading={pending === "soul"}
            onClick={() => void handleSoulSave()}
          >
            {pending === "soul" ? "保存中…" : "保存 SOUL"}
          </Button>
        </div>
        <div className="section-card-body">
          <Textarea rows={14} mono value={soul} onChange={(event) => setSoul(event.target.value)} aria-label="SOUL.md" />
        </div>
      </section>

      {/* USER.md */}
      <section className="section-card">
        <div className="section-card-head">
          <h3 style={{ fontSize: 15, fontWeight: 600 }}>USER.md · 用户上下文模板</h3>
          <Button
            variant="primary"
            size="sm"
            disabled={pending === "user"}
            loading={pending === "user"}
            onClick={() => void handleUserSave()}
          >
            {pending === "user" ? "保存中…" : "保存 USER"}
          </Button>
        </div>
        <div className="section-card-body">
          <Textarea rows={14} mono value={userContent} onChange={(event) => setUserContent(event.target.value)} aria-label="USER.md" />
        </div>
      </section>

      {error ? (
        <div
          role="alert"
          className="card"
          style={{
            padding: "10px 12px",
            borderColor: "color-mix(in oklch, var(--danger) 25%, var(--border))",
            background: "var(--danger-soft)",
            color: "var(--danger)",
            fontSize: 13,
          }}
        >
          {error}
        </div>
      ) : null}
    </div>
  );
}
