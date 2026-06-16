"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import {
  apiErrorMessage,
  archiveAdminTemplate,
  listAdminTemplates,
  type Template,
} from "@/lib/api";

export default function AdminTemplatesPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, user } = useSessionState();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [confirmingTemplateId, setConfirmingTemplateId] = useState("");
  const [pendingTemplateId, setPendingTemplateId] = useState("");

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
      const response = await listAdminTemplates(apiClient);
      if (!active) {
        return;
      }
      if (!response.ok) {
        setErrorStatus(response.status);
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      setTemplates(response.data.templates);
    })();

    return () => {
      active = false;
    };
  }, [apiClient, loading, router, user]);

  async function handleArchive(templateId: string) {
    setPendingTemplateId(templateId);
    setError("");
    const response = await archiveAdminTemplate(apiClient, templateId);
    setPendingTemplateId("");
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    setConfirmingTemplateId("");
    setTemplates((current) => current.filter((template) => template.id !== templateId));
  }

  return (
    <section className="grid gap-6">
      <div className="panel rounded-[2rem] p-8">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p className="eyebrow">Admin Templates</p>
            <h1 className="mt-4 text-4xl font-semibold tracking-tight text-stone-950">
              Draft, publish, and clone template versions.
            </h1>
          </div>
          <Link
            className="rounded-full bg-stone-950 px-5 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)]"
            href="/admin/templates/new"
          >
            New Draft
          </Link>
        </div>
      </div>
      {error ? (
        <ApiErrorState message={error} status={errorStatus} />
      ) : null}
      <div className="grid gap-5 lg:grid-cols-2">
        {templates.map((template) => (
          <div className="panel rounded-[1.75rem] p-6" key={template.id}>
            <div className="flex items-center justify-between gap-4">
              <Link
                className="min-w-0 flex-1 hover:-translate-y-0.5"
                href={`/admin/templates/${template.id}`}
              >
                <div className="flex items-center justify-between gap-4">
                  <h2 className="text-2xl font-semibold text-stone-950">{template.name}</h2>
                  <span className="rounded-full bg-stone-900 px-3 py-1 text-xs uppercase tracking-[0.18em] text-stone-50">
                    {template.status}
                  </span>
                </div>
                <p className="mt-3 text-sm leading-7 text-stone-600">
                  {template.description || "No description provided."}
                </p>
                <p className="mt-5 text-xs uppercase tracking-[0.18em] text-stone-500">
                  Version {template.version}
                </p>
              </Link>
            </div>
            <div className="mt-5 flex flex-wrap items-center gap-3">
              {confirmingTemplateId === template.id ? (
                <>
                  <button
                    className="rounded-full border border-red-300 bg-red-50 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-red-700 disabled:cursor-not-allowed disabled:opacity-60"
                    disabled={pendingTemplateId === template.id}
                    onClick={() => void handleArchive(template.id)}
                    type="button"
                  >
                    {pendingTemplateId === template.id ? "Deleting..." : "Confirm Delete"}
                  </button>
                  <button
                    className="rounded-full border border-stone-900/15 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-stone-700"
                    disabled={pendingTemplateId === template.id}
                    onClick={() => setConfirmingTemplateId("")}
                    type="button"
                  >
                    Cancel
                  </button>
                </>
              ) : (
                <button
                  className="rounded-full border border-red-300 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-red-700 hover:bg-red-50"
                  disabled={pendingTemplateId === template.id}
                  onClick={() => setConfirmingTemplateId(template.id)}
                  type="button"
                >
                  Delete Template
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
