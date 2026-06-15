"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import { apiErrorMessage, listAgents, type Agent } from "@/lib/api";

export default function AgentsPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, user } = useSessionState();
  const [agents, setAgents] = useState<Agent[]>([]);
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
      const response = await listAgents(apiClient);
      if (!active) {
        return;
      }
      if (!response.ok) {
        setErrorStatus(response.status);
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      setAgents(response.data.agents);
    })();

    return () => {
      active = false;
    };
  }, [apiClient, loading, router, user]);

  return (
    <section className="grid gap-6">
      <div className="panel rounded-[2rem] p-8">
        <p className="eyebrow">Agents</p>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight text-stone-950">
          Follow runtime progress and pairing state.
        </h1>
      </div>
      {error ? (
        <ApiErrorState message={error} status={errorStatus} />
      ) : null}
      <div className="grid gap-5 lg:grid-cols-2">
        {agents.map((agent) => (
          <Link
            className="panel rounded-[1.75rem] p-6 hover:-translate-y-0.5"
            href={`/agents/${agent.id}`}
            key={agent.id}
          >
            <div className="flex items-center justify-between gap-4">
              <p className="text-xl font-semibold text-stone-950">{agent.name}</p>
              <span className="rounded-full bg-stone-900 px-3 py-1 text-xs uppercase tracking-[0.18em] text-stone-50">
                {agent.status}
              </span>
            </div>
            <p className="mt-3 text-sm text-stone-600">Template {agent.templateVersion}</p>
            {agent.lastErrorCode ? (
              <p className="mt-4 text-sm text-red-700">{agent.lastErrorCode}</p>
            ) : null}
          </Link>
        ))}
      </div>
    </section>
  );
}
