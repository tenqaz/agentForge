"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { useApiClient, useSessionState } from "@/components/app-shell";
import Breadcrumbs from "@/components/ui/breadcrumbs";
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
    <>
      <Breadcrumbs
        items={[
          { label: "模板管理", href: "/admin/templates" },
          { label: template?.name ?? "模板" },
        ]}
      />

      <div className="page-head">
        <div>
          <span className="meta">编辑模板</span>
          <h1>{template?.name ?? "加载中…"}</h1>
          {template ? (
            <div className="row" style={{ gap: 8, marginTop: 8 }}>
              <StatusChip kind="template" value={template.status} />
              <span className="tag tag-mono">v{template.version}</span>
            </div>
          ) : null}
        </div>
      </div>

      {error ? (
        <div
          role="alert"
          className="card"
          style={{
            padding: "12px 16px",
            borderColor: "color-mix(in oklch, var(--danger) 25%, var(--border))",
            background: "var(--danger-soft)",
            color: "var(--danger)",
            fontSize: 14,
          }}
        >
          {error}
        </div>
      ) : null}

      {template ? (
        <div className="grid-2-1">
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
    </>
  );
}
