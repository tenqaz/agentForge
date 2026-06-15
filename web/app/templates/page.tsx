"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import { apiErrorMessage, listPublishedTemplates, type Template } from "@/lib/api";

export default function TemplatesPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, user } = useSessionState();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");

  useEffect(() => {
    if (loading) {
      return;
    }
    if (!user) {
      router.replace("/login");
      return;
    }

    let active = true;
    void (async () => {
      const response = await listPublishedTemplates(apiClient);
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

  return (
    <section className="grid gap-6">
      <div className="panel rounded-[2rem] p-8">
        <p className="eyebrow">Published Templates</p>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight text-stone-950">
          Pick a template and launch an agent.
        </h1>
        <p className="mt-3 max-w-2xl text-base leading-7 text-stone-600">
          Only published templates appear here. Creating an agent immediately queues runtime
          provisioning in the backend.
        </p>
      </div>
      {error ? (
        <ApiErrorState message={error} status={errorStatus} />
      ) : null}
      <div className="grid gap-5 lg:grid-cols-2">
        {templates.map((template) => (
          <Link
            className="panel rounded-[1.75rem] p-6 hover:-translate-y-0.5"
            href={`/templates/${template.id}`}
            key={template.id}
          >
            <div className="flex items-center justify-between gap-4">
              <p className="eyebrow">Version {template.version}</p>
              <span className="rounded-full bg-stone-900 px-3 py-1 text-xs uppercase tracking-[0.18em] text-stone-50">
                {template.status}
              </span>
            </div>
            <h2 className="mt-4 text-2xl font-semibold text-stone-950">{template.name}</h2>
            <p className="mt-3 text-sm leading-7 text-stone-600">
              {template.description || "No description provided."}
            </p>
          </Link>
        ))}
      </div>
    </section>
  );
}
