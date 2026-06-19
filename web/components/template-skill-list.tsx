"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { useApiClient } from "@/components/app-shell";
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
    <div className="panel rounded-[1.75rem] p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="eyebrow">Skills</p>
          <h2 className="mt-3 text-2xl font-semibold text-stone-950">Upload or delete skills</h2>
          <p className="mt-2 text-sm leading-7 text-stone-600">
            Upload a skill ZIP with one top-level directory and a `SKILL.md` inside it.
          </p>
        </div>
      </div>
      <div className="mt-6 grid gap-4">
        {skills.map((skill) => (
          <div
            className="rounded-[1.25rem] border border-stone-900/10 bg-white/85 px-4 py-4"
            key={skill.id}
          >
            <div className="flex items-center justify-between gap-4">
              <div>
                <p className="text-lg font-semibold text-stone-950">{skill.skillName}</p>
                <p className="mt-1 font-mono text-xs text-stone-500">{skill.checksum}</p>
              </div>
              <button
                className="rounded-full border border-stone-900/15 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-stone-700 hover:border-stone-900 hover:bg-stone-900 hover:text-stone-50 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={pendingSkillId === skill.id || busySection === "publication"}
                onClick={() => void handleDeleteSkill(skill.id)}
                type="button"
              >
                {pendingSkillId === skill.id ? "Deleting..." : "Delete"}
              </button>
            </div>
          </div>
        ))}
      </div>
      <div className="mt-6 grid gap-4 rounded-[1.5rem] border border-dashed border-stone-900/15 bg-stone-50/70 p-5">
        <label className="grid gap-2 text-sm font-medium text-stone-700">
          Skill ZIP
          <input
            accept=".zip,application/zip"
            className="rounded-[1.1rem] border border-stone-900/12 bg-white px-4 py-3 text-base text-stone-950 shadow-sm"
            onChange={(event) => setSkillFile(event.target.files?.[0] ?? null)}
            type="file"
          />
        </label>
        <p className="text-sm leading-6 text-stone-600">
          The ZIP must contain one top-level folder named after the skill, and that folder must
          include `SKILL.md`.
        </p>
        <button
          className="w-fit rounded-full bg-stone-950 px-5 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)] disabled:cursor-not-allowed disabled:opacity-60"
          disabled={
            pendingSkillId === "create" || !skillFile || busySection === "publication"
          }
          onClick={() => void handleAddSkill()}
          type="button"
        >
          {pendingSkillId === "create" ? "Uploading..." : "Upload Skill"}
        </button>
      </div>
      {error ? (
        <div className="mt-4 rounded-[1.25rem] border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      ) : null}
    </div>
  );
}
