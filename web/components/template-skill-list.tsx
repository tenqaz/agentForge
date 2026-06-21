"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Trash2, Upload } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
import {
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
  const [skillFile, setSkillFile] = useState<File | null>(null);
  const [pendingSkillId, setPendingSkillId] = useState("");
  const [error, setError] = useState("");

  function moveTemplate(nextTemplate: Template) {
    setTemplate(nextTemplate);
    onTemplateShift(nextTemplate);
    if (nextTemplate.id !== template.id) {
      router.replace(`/admin/templates/${nextTemplate.id}`);
    }
  }

  async function handleAddSkill() {
    setPendingSkillId("create");
    onBusySectionChange("skills");
    setError("");
    if (!skillFile) {
      setPendingSkillId("");
      onBusySectionChange("");
      return;
    }
    const response = await addTemplateSkill(apiClient, template.id, skillFile);
    setPendingSkillId("");
    onBusySectionChange("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    const nextSkills = [...skills, response.data.skill];
    setSkills(nextSkills);
    onSkillsChange(nextSkills);
    setSkillFile(null);
    if (response.data.skill.templateId !== template.id) {
      moveTemplate({ ...template, id: response.data.skill.templateId });
    }
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
            ZIP 必须包含一个以技能名命名的顶层目录，且目录内有 SKILL.md。
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
              <label className="field-label" htmlFor="skill-zip-input">Skill ZIP</label>
              <input
                id="skill-zip-input"
                type="file"
                accept=".zip,application/zip"
                onChange={(event) => setSkillFile(event.target.files?.[0] ?? null)}
                className="input"
                style={{ padding: "8px 12px" }}
              />
            </div>
            <div className="form-actions">
              <Button
                variant="primary"
                size="sm"
                leftIcon={<Upload size={14} strokeWidth={1.75} />}
                disabled={pendingSkillId === "create" || !skillFile || busySection === "publication"}
                loading={pendingSkillId === "create"}
                onClick={() => void handleAddSkill()}
              >
                {pendingSkillId === "create" ? "上传中…" : "上传 Skill"}
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
