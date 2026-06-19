"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { useApiClient, useSessionState } from "@/components/app-shell";
import { Card, CardTitle } from "@/components/ui/card";
import StatusChip from "@/components/ui/status-chip";
import TemplateEditor from "@/components/template-editor";
import TemplateSkillList from "@/components/template-skill-list";
import {
  apiErrorMessage,
  getAdminTemplate,
  getTemplateSoul,
  getTemplateUser,
  listTemplateSkills,
  type Skill,
  type Template,
} from "@/lib/api";

export default function AdminTemplateDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [template, setTemplate] = useState<Template | null>(null);
  const [soul, setSoul] = useState("");
  const [userContent, setUserContent] = useState("");
  const [skills, setSkills] = useState<Skill[]>([]);
  const [error, setError] = useState("");
  const [busySection, setBusySection] = useState("");

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
      const templateResponse = await getAdminTemplate(apiClient, id);
      if (!templateResponse.ok) {
        if (active) {
          setError(apiErrorMessage(templateResponse.error.code, templateResponse.error.message));
        }
        return;
      }
      const currentTemplate = templateResponse.data.template;

      const [soulResponse, userResponse, skillsResponse] = await Promise.all([
        getTemplateSoul(apiClient, currentTemplate.id),
        getTemplateUser(apiClient, currentTemplate.id),
        listTemplateSkills(apiClient, currentTemplate.id),
      ]);

      if (!active) {
        return;
      }
      if (!soulResponse.ok || !userResponse.ok || !skillsResponse.ok) {
        const failed = [soulResponse, userResponse, skillsResponse].find(
          (response) => !response.ok,
        );
        if (failed && !failed.ok) {
          setError(apiErrorMessage(failed.error.code, failed.error.message));
        }
        return;
      }

      setTemplate(currentTemplate);
      setSoul(soulResponse.data.content);
      setUserContent(userResponse.data.content);
      setSkills(skillsResponse.data.skills);
    })();

    return () => {
      active = false;
    };
  }, [apiClient, id, sessionLoading, router, user]);

  return (
    <section className="flex flex-col gap-6">
      <Card>
        <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
          Template 详情
        </p>
        <CardTitle as="h1" className="mt-2 text-3xl">
          {template?.name ?? "加载中..."}
        </CardTitle>
        {template ? (
          <div className="mt-3 flex items-center gap-2">
            <StatusChip kind="template" value={template.status} />
            <span className="font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
              v{template.version}
            </span>
          </div>
        ) : null}
      </Card>

      {error ? (
        <div
          role="alert"
          className="rounded-[var(--radius-xl)] border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-soft)] px-4 py-3 text-sm text-[color:var(--color-danger)]"
        >
          {error}
        </div>
      ) : null}

      {template ? (
        <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
          <TemplateEditor
            busySection={busySection}
            initialSoul={soul}
            initialTemplate={template}
            initialUserContent={userContent}
            onBusySectionChange={setBusySection}
            onTemplateChange={(nextTemplate, nextSoul, nextUser) => {
              setTemplate(nextTemplate);
              setSoul(nextSoul);
              setUserContent(nextUser);
            }}
          />
          <TemplateSkillList
            busySection={busySection}
            initialSkills={skills}
            initialTemplate={template}
            onBusySectionChange={setBusySection}
            onSkillsChange={setSkills}
            onTemplateShift={setTemplate}
          />
        </div>
      ) : null}
    </section>
  );
}
