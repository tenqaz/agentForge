"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Trash2, Upload } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
import { Card, CardDescription, CardTitle } from "@/components/ui/card";
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
    <Card>
      <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
        Skills
      </p>
      <CardTitle as="h2" className="mt-1.5">
        上传或删除技能
      </CardTitle>
      <CardDescription>
        ZIP 必须包含一个以技能名命名的顶层目录，且目录内有 SKILL.md。
      </CardDescription>

      <div className="mt-5 grid gap-2">
        {skills.map((skill) => (
          <div
            key={skill.id}
            className="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg)]/40 px-3.5 py-3"
          >
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-[color:var(--color-fg)]">
                {skill.skillName}
              </p>
              <p className="mt-0.5 truncate font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
                {skill.checksum}
              </p>
            </div>
            <Button
              variant="ghost"
              size="sm"
              leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
              disabled={pendingSkillId === skill.id || busySection === "publication"}
              loading={pendingSkillId === skill.id}
              onClick={() => void handleDeleteSkill(skill.id)}
              className="text-[color:var(--color-danger)] hover:bg-[color:var(--color-danger-soft)] hover:text-[color:var(--color-danger)]"
            >
              {pendingSkillId === skill.id ? "删除中..." : "删除"}
            </Button>
          </div>
        ))}
      </div>

      <div className="mt-6 grid gap-3 rounded-[var(--radius-lg)] border border-dashed border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg)]/40 p-4">
        <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
          Skill ZIP
          <input
            type="file"
            accept=".zip,application/zip"
            onChange={(event) => setSkillFile(event.target.files?.[0] ?? null)}
            className="block w-full rounded-[var(--radius-md)] border border-[color:var(--color-border-default)] bg-[color:var(--color-bg-input)] px-3 py-2 text-sm text-[color:var(--color-fg)] hover:border-[color:var(--color-border-strong)] file:mr-3 file:rounded-[var(--radius-md)] file:border-0 file:bg-[color:var(--color-bg-hover)] file:px-3 file:py-1.5 file:text-xs file:font-medium file:text-[color:var(--color-fg)] hover:file:bg-[color:var(--color-bg-active)]"
          />
        </label>
        <Button
          variant="primary"
          size="sm"
          leftIcon={<Upload size={14} strokeWidth={1.75} />}
          disabled={
            pendingSkillId === "create" || !skillFile || busySection === "publication"
          }
          loading={pendingSkillId === "create"}
          onClick={() => void handleAddSkill()}
          className="w-fit"
        >
          {pendingSkillId === "create" ? "上传中..." : "上传 Skill"}
        </Button>
      </div>

      {error ? (
        <div
          role="alert"
          className="mt-4 rounded-[var(--radius-md)] border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-soft)] px-3.5 py-2.5 text-sm text-[color:var(--color-danger)]"
        >
          {error}
        </div>
      ) : null}
    </Card>
  );
}
