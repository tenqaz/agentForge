"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Plus } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Breadcrumbs from "@/components/ui/breadcrumbs";
import Button from "@/components/ui/button";
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
    <>
      <Breadcrumbs
        items={[
          { label: "模板管理", href: "/admin/templates" },
          { label: "新建草稿" },
        ]}
      />

      <div className="page-head">
        <div>
          <span className="meta">新建草稿</span>
          <h1>创建一个新的模板草稿</h1>
          <p className="lead">填写元数据与初始 SOUL/USER 文件，可选择上传初始 Skill ZIP。</p>
        </div>
      </div>

      <section className="section-card">
        <div className="section-card-body">
          <div className="form-stack" style={{ maxWidth: 720 }}>
            <div className="field">
              <label className="field-label" htmlFor="tpl-name">名称</label>
              <Input
                id="tpl-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="例如：客户支持"
              />
            </div>
            <div className="field">
              <label className="field-label" htmlFor="tpl-desc">描述</label>
              <Textarea
                id="tpl-desc"
                rows={3}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="一句话说明这个模板的用途"
              />
            </div>
            <div className="field">
              <label className="field-label" htmlFor="tpl-soul">SOUL.md</label>
              <Textarea
                id="tpl-soul"
                rows={10}
                mono
                value={soulContent}
                onChange={(event) => setSoulContent(event.target.value)}
                placeholder="# Persona ..."
              />
            </div>
            <div className="field">
              <label className="field-label" htmlFor="tpl-user">USER.md</label>
              <Textarea
                id="tpl-user"
                rows={8}
                mono
                value={userContent}
                onChange={(event) => setUserContent(event.target.value)}
                placeholder="# User ..."
              />
            </div>
            <div className="field">
              <label className="field-label" htmlFor="tpl-skills">Skill ZIP</label>
              <input
                id="tpl-skills"
                type="file"
                accept=".zip,application/zip"
                multiple
                onChange={(event) => setSkillZips(Array.from(event.target.files ?? []))}
                className="input"
                style={{ padding: "8px 12px" }}
              />
            </div>
            {skillZips.length > 0 ? (
              <div
                className="card"
                style={{ padding: "10px 12px", background: "var(--surface-2)" }}
              >
                <span className="meta">已选择 {skillZips.length} 个文件</span>
                {skillZips.map((file) => (
                  <span className="meta num" key={file.name} style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {file.name}
                  </span>
                ))}
              </div>
            ) : null}

            {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

            <div className="form-actions">
              <Button
                variant="primary"
                leftIcon={<Plus size={16} strokeWidth={1.75} />}
                disabled={pending || !name.trim() || !soulContent.trim()}
                loading={pending}
                onClick={() => void handleCreate()}
              >
                {pending ? "创建中…" : "创建草稿"}
              </Button>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
