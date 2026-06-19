"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Plus } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Button from "@/components/ui/button";
import { Card, CardDescription, CardTitle } from "@/components/ui/card";
import Input from "@/components/ui/input";
import Textarea from "@/components/ui/textarea";
import { apiErrorMessage, createAdminTemplate } from "@/lib/api";

export default function NewAdminTemplatePage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [soulContent, setSoulContent] = useState("");
  const [userContent, setUserContent] = useState("");
  const [skillZips, setSkillZips] = useState<File[]>([]);
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
    if (user.role !== "admin") {
      router.replace("/templates");
    }
  }, [sessionLoading, router, user]);

  async function handleCreate() {
    setPending(true);
    setError("");
    const formData = new FormData();
    formData.set("name", name);
    formData.set("description", description);
    formData.set("soulContent", soulContent);
    formData.set("userContent", userContent);
    for (const skillZip of skillZips) {
      formData.append("skillZips", skillZip, skillZip.name);
    }
    const response = await createAdminTemplate(apiClient, formData);
    setPending(false);
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    router.push(`/admin/templates/${response.data.template.id}`);
  }

  return (
    <section className="mx-auto flex max-w-3xl flex-col gap-6">
      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
          新建草稿
        </p>
        <CardTitle as="h1" className="mt-2 text-3xl">
          创建一个新的 Template 草稿
        </CardTitle>
        <CardDescription>
          填写元数据与初始 SOUL/USER 文件，可选择上传初始 Skill ZIP。
        </CardDescription>

        <div className="mt-6 grid gap-4">
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            名称
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如：客户支持"
            />
          </label>
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            描述
            <Textarea
              rows={3}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="一句话说明这个 Template 的用途"
            />
          </label>
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            SOUL.md
            <Textarea
              rows={10}
              mono
              value={soulContent}
              onChange={(event) => setSoulContent(event.target.value)}
              placeholder="# Persona ..."
            />
          </label>
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            USER.md
            <Textarea
              rows={8}
              mono
              value={userContent}
              onChange={(event) => setUserContent(event.target.value)}
              placeholder="# User ..."
            />
          </label>
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            Skill ZIP
            <input
              type="file"
              accept=".zip,application/zip"
              multiple
              onChange={(event) => setSkillZips(Array.from(event.target.files ?? []))}
              className="block w-full rounded-[var(--radius-md)] border border-[color:var(--color-border-default)] bg-[color:var(--color-bg-input)] px-3 py-2 text-sm text-[color:var(--color-fg)] hover:border-[color:var(--color-border-strong)] file:mr-3 file:rounded-[var(--radius-md)] file:border-0 file:bg-[color:var(--color-bg-hover)] file:px-3 file:py-1.5 file:text-xs file:font-medium file:text-[color:var(--color-fg)] hover:file:bg-[color:var(--color-bg-active)]"
            />
          </label>
          {skillZips.length > 0 ? (
            <div className="rounded-[var(--radius-md)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg)]/40 px-3 py-2.5 font-mono text-[11px] text-[color:var(--color-fg-muted)]">
              <p className="mb-1 text-[color:var(--color-fg-subtle)]">已选择 {skillZips.length} 个文件</p>
              {skillZips.map((file) => (
                <p key={file.name} className="truncate">
                  {file.name}
                </p>
              ))}
            </div>
          ) : null}

          {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

          <Button
            variant="primary"
            leftIcon={<Plus size={16} strokeWidth={1.75} />}
            disabled={pending || !name.trim() || !soulContent.trim()}
            loading={pending}
            onClick={() => void handleCreate()}
            className="w-fit"
          >
            {pending ? "创建中..." : "创建草稿"}
          </Button>
        </div>
      </Card>
    </section>
  );
}
