"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { useApiClient, useSessionState } from "@/components/app-shell";
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
  const { loading, user } = useSessionState();
  const [template, setTemplate] = useState<Template | null>(null);
  const [soul, setSoul] = useState("");
  const [userContent, setUserContent] = useState("");
  const [skills, setSkills] = useState<Skill[]>([]);
  const [error, setError] = useState("");
  const [busySection, setBusySection] = useState("");

  useEffect(() => {
    if (loading) {
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
  }, [apiClient, id, loading, router, user]);

  return (
    <section className="grid gap-6">
      <div className="panel rounded-[2rem] p-8">
        <p className="eyebrow">Template Detail</p>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight text-stone-950">
          {template?.name ?? "Loading template..."}
        </h1>
      </div>
      {error ? (
        <div className="rounded-[1.5rem] border border-red-300 bg-red-50 px-5 py-4 text-sm text-red-700">
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
