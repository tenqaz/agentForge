"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import {
  apiErrorMessage,
  createAgent,
  getPublishedTemplate,
  type Template,
} from "@/lib/api";

export default function TemplateDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, user } = useSessionState();
  const [template, setTemplate] = useState<Template | null>(null);
  const [agentName, setAgentName] = useState("");
  const [pending, setPending] = useState(false);
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
      const response = await getPublishedTemplate(apiClient, id);
      if (!active) {
        return;
      }
      if (!response.ok) {
        setErrorStatus(response.status);
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      setTemplate(response.data.template);
      setAgentName(`${response.data.template.name} Agent`);
    })();

    return () => {
      active = false;
    };
  }, [apiClient, id, loading, router, user]);

  async function handleCreateAgent() {
    if (!template) {
      return;
    }
    setPending(true);
    setError("");
    const response = await createAgent(apiClient, template.id, agentName.trim());
    setPending(false);
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    router.push(`/agents/${response.data.agent.id}`);
  }

  return (
    <section className="grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
      <div className="panel rounded-[2rem] p-8">
        <p className="eyebrow">Template Detail</p>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight text-stone-950">
          {template?.name ?? "Loading template..."}
        </h1>
        <p className="mt-4 text-base leading-8 text-stone-600">
          {template?.description || "No description provided for this template."}
        </p>
      </div>
      <div className="panel rounded-[2rem] p-8">
        <p className="eyebrow">Create Agent</p>
        <label className="mt-5 grid gap-2 text-sm font-medium text-stone-700">
          Agent name
          <input
            className="rounded-[1.25rem] border border-stone-900/12 bg-white px-4 py-3 text-base text-stone-950 shadow-sm"
            onChange={(event) => setAgentName(event.target.value)}
            value={agentName}
          />
        </label>
        {error ? (
          <div className="mt-4">
            <ApiErrorState message={error} status={errorStatus} />
          </div>
        ) : null}
        <button
          className="mt-6 rounded-full bg-stone-950 px-6 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)] disabled:cursor-not-allowed disabled:opacity-60"
          disabled={pending || !template || !agentName.trim()}
          onClick={() => void handleCreateAgent()}
          type="button"
        >
          {pending ? "Creating Agent..." : "Create Agent"}
        </button>
      </div>
    </section>
  );
}
