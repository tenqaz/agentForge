"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Trash2, Upload } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
import {
  MAX_SKILLS_PER_TEMPLATE,
  addTemplateSkill,
  apiErrorMessage,
  deleteTemplateSkill,
  type Skill,
  type Template,
} from "@/lib/api";

export default function TemplateSkillList({
  initialTemplate,
  initialSkills,
  busySection,
  onBusySectionChange,
  onTemplateShift,
  onSkillsChange,
}: {
  initialTemplate: Template;
  initialSkills: Skill[];
  busySection: string;
  onBusySectionChange: (section: string) => void;
  onTemplateShift: (template: Template) => void;
  onSkillsChange: (skills: Skill[]) => void;
}) {
  const apiClient = useApiClient();
  const router = useRouter();
  const [template, setTemplate] = useState(initialTemplate);
  const [skills, setSkills] = useState(initialSkills);
  const [skillFiles, setSkillFiles] = useState<File[]>([]);
  const [pendingSkillId, setPendingSkillId] = useState("");
  const [uploadProgress, setUploadProgress] = useState<{ current: number; total: number } | null>(null);
  const [error, setError] = useState("");

  const remainingSlots = Math.max(0, MAX_SKILLS_PER_TEMPLATE - skills.length);
  const limitReached = remainingSlots === 0;

  function moveTemplate(nextTemplate: Template) {
    setTemplate(nextTemplate);
    onTemplateShift(nextTemplate);
    if (nextTemplate.id !== template.id) {
      router.replace(`/admin/templates/${nextTemplate.id}`);
    }
  }

  async function handleAddSkill() {
    if (skillFiles.length === 0) {
      return;
    }
    if (skills.length + skillFiles.length > MAX_SKILLS_PER_TEMPLATE) {
      setError(
        `每个模板最多 ${MAX_SKILLS_PER_TEMPLATE} 个技能，当前已有 ${skills.length} 个，最多还能再上传 ${remainingSlots} 个。`,
      );
      return;
    }

    setPendingSkillId("create");
    onBusySectionChange("skills");
    setError("");

    let currentTemplate = template;
    let currentSkills = skills;
    let succeeded = 0;

    for (let i = 0; i < skillFiles.length; i++) {
      const file = skillFiles[i];
      setUploadProgress({ current: i + 1, total: skillFiles.length });
      const response = await addTemplateSkill(apiClient, currentTemplate.id, file);
      if (!response.ok) {
        setError(
          `已成功上传 ${succeeded} 个，第 ${i + 1} 个（${file.name}）失败：${apiErrorMessage(
            response.error.code,
            response.error.message,
          )}`,
        );
        break;
      }
      const newSkill = response.data.skill;
      // 后端可能在第一次写入时把 published 模板克隆成新 draft，需要同步切换。
      if (newSkill.templateId !== currentTemplate.id) {
        currentTemplate = { ...currentTemplate, id: newSkill.templateId };
        moveTemplate(currentTemplate);
      }
      currentSkills = [...currentSkills, newSkill];
      setSkills(currentSkills);
      onSkillsChange(currentSkills);
      succeeded += 1;
    }

    setPendingSkillId("");
    setUploadProgress(null);
    onBusySectionChange("");
    setSkillFiles([]);
  }

  async function handleDeleteSkill(skillId: string) {
    setPendingSkillId(skillId);
    onBusySectionChange("skills");
    setError("");
    const response = await deleteTemplateSkill(apiClient, template.id, skillId);
    setPendingSkillId("");
    onBusySectionChange("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    const nextSkills = skills.filter((skill) => skill.id !== skillId);
    setSkills(nextSkills);
    onSkillsChange(nextSkills);
    if (response.status === 200 && response.data?.template) {
      moveTemplate(response.data.template);
    }
  }

  return (
    <section className="section-card">
      <div className="section-card-head">
        <div>
          <span className="meta">Skills</span>
          <h3 style={{ fontSize: 15, fontWeight: 600, marginTop: 4 }}>上传或删除技能</h3>
          <p className="muted" style={{ fontSize: 13, marginTop: 4 }}>
            ZIP 必须包含一个以技能名命名的顶层目录，且目录内有 SKILL.md。每个模板最多 {MAX_SKILLS_PER_TEMPLATE} 个技能。
          </p>
        </div>
      </div>

      <div className="section-card-body">
        <div className="list" style={{ marginBottom: 20 }}>
          {skills.map((skill) => (
            <div className="list-item" key={skill.id}>
              <span className="avatar" style={{ width: 32, height: 32, fontSize: 12 }} aria-hidden="true">
                {skill.skillName.slice(0, 1).toUpperCase()}
              </span>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontWeight: 500, fontSize: 14, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {skill.skillName}
                </div>
                <div className="meta" style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {skill.checksum}
                </div>
              </div>
              <Button
                variant="ghost"
                size="sm"
                leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
                disabled={pendingSkillId === skill.id || busySection === "publication"}
                loading={pendingSkillId === skill.id}
                onClick={() => void handleDeleteSkill(skill.id)}
                style={{ color: "var(--danger)" }}
              >
                {pendingSkillId === skill.id ? "删除中…" : "删除"}
              </Button>
            </div>
          ))}
          {skills.length === 0 ? (
            <div className="empty" style={{ padding: "24px 16px" }}>
              <p>还没有技能，上传第一个 Skill ZIP。</p>
            </div>
          ) : null}
        </div>

        <div
          className="card"
          style={{
            padding: 16,
            borderStyle: "dashed",
            borderColor: "var(--border-strong)",
            background: "transparent",
          }}
        >
          <div className="form-stack">
            <div className="field">
              <label className="field-label" htmlFor="skill-zip-input">
                Skill ZIP {limitReached ? "（已达上限）" : `（剩余配额 ${remainingSlots}）`}
              </label>
              <input
                id="skill-zip-input"
                type="file"
                accept=".zip,application/zip"
                multiple
                disabled={limitReached || pendingSkillId === "create"}
                onChange={(event) => {
                  const picked = Array.from(event.target.files ?? []);
                  if (picked.length > remainingSlots) {
                    setError(
                      `本次最多还能上传 ${remainingSlots} 个技能，但选择了 ${picked.length} 个。`,
                    );
                    setSkillFiles([]);
                    event.target.value = "";
                    return;
                  }
                  setError("");
                  setSkillFiles(picked);
                }}
                className="input"
                style={{ padding: "8px 12px" }}
              />
            </div>
            {skillFiles.length > 0 ? (
              <div
                className="card"
                style={{ padding: "10px 12px", background: "var(--surface-2)" }}
              >
                <span className="meta">已选择 {skillFiles.length} 个文件</span>
                {skillFiles.map((file) => (
                  <span
                    className="meta num"
                    key={file.name}
                    style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
                  >
                    {file.name}
                  </span>
                ))}
              </div>
            ) : null}
            <div className="form-actions">
              <Button
                variant="primary"
                size="sm"
                leftIcon={<Upload size={14} strokeWidth={1.75} />}
                disabled={
                  pendingSkillId === "create" ||
                  skillFiles.length === 0 ||
                  busySection === "publication" ||
                  limitReached
                }
                loading={pendingSkillId === "create"}
                onClick={() => void handleAddSkill()}
              >
                {pendingSkillId === "create"
                  ? uploadProgress
                    ? `上传中… (${uploadProgress.current}/${uploadProgress.total})`
                    : "上传中…"
                  : `上传 Skill${skillFiles.length > 1 ? `（${skillFiles.length}）` : ""}`}
              </Button>
            </div>
          </div>
        </div>

        {error ? (
          <div
            role="alert"
            className="card"
            style={{
              marginTop: 16,
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
    </section>
  );
}
