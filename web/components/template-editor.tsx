"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Trash2 } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
import { Card, CardTitle } from "@/components/ui/card";
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
    const response = await updateAdminTemplate(
      apiClient,
      template.id,
      name,
      description,
    );
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
      ? "取消发布中..."
      : "取消发布"
    : pending === "publication"
      ? "发布中..."
      : "发布";

  return (
    <div className="grid gap-6">
      {/* Metadata + publication */}
      <Card>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
              Template
            </p>
            <CardTitle as="h2" className="mt-1.5">
              元数据与发布
            </CardTitle>
            <div className="mt-3 flex items-center gap-2">
              <StatusChip kind="template" value={template.status} />
              <span className="font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
                v{template.version}
              </span>
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button
              variant={isPublished ? "secondary" : "primary"}
              size="sm"
              disabled={
                pending === "publication" ||
                pending === "archive" ||
                busySection === "skills"
              }
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
                  disabled={pending === "archive" || busySection === "skills"}
                  loading={pending === "archive"}
                  onClick={() => void handleArchive()}
                >
                  {pending === "archive" ? "删除中..." : "确认删除"}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={pending === "archive" || busySection === "skills"}
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
                disabled={
                  pending === "publication" ||
                  pending === "archive" ||
                  busySection === "skills"
                }
                onClick={() => setConfirmArchive(true)}
                className="text-[color:var(--color-danger)] hover:bg-[color:var(--color-danger-soft)] hover:text-[color:var(--color-danger)]"
              >
                删除 Template
              </Button>
            )}
          </div>
        </div>

        <div className="mt-5 grid gap-4">
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            名称
            <Input value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            描述
            <Textarea
              rows={3}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
          </label>
          <Button
            variant="primary"
            size="sm"
            disabled={pending === "metadata"}
            loading={pending === "metadata"}
            onClick={() => void handleMetadataSave()}
            className="w-fit"
          >
            {pending === "metadata" ? "保存中..." : "保存元数据"}
          </Button>
        </div>
      </Card>

      {/* SOUL.md */}
      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
          SOUL.md
        </p>
        <Textarea
          rows={14}
          mono
          className="mt-3"
          value={soul}
          onChange={(event) => setSoul(event.target.value)}
          aria-label="SOUL.md"
        />
        <Button
          variant="primary"
          size="sm"
          className="mt-4 w-fit"
          disabled={pending === "soul"}
          loading={pending === "soul"}
          onClick={() => void handleSoulSave()}
        >
          {pending === "soul" ? "保存中..." : "保存 SOUL"}
        </Button>
      </Card>

      {/* USER.md */}
      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
          USER.md
        </p>
        <Textarea
          rows={14}
          mono
          className="mt-3"
          value={userContent}
          onChange={(event) => setUserContent(event.target.value)}
          aria-label="USER.md"
        />
        <Button
          variant="primary"
          size="sm"
          className="mt-4 w-fit"
          disabled={pending === "user"}
          loading={pending === "user"}
          onClick={() => void handleUserSave()}
        >
          {pending === "user" ? "保存中..." : "保存 USER"}
        </Button>
      </Card>

      {error ? (
        <div
          role="alert"
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-soft)] px-3.5 py-2.5 text-sm text-[color:var(--color-danger)]"
        >
          {error}
        </div>
      ) : null}
    </div>
  );
}
