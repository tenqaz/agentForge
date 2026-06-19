"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import { apiErrorMessage, deleteAgent, listAgents, type Agent } from "@/lib/api";

export default function AgentsPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, user } = useSessionState();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [confirmingId, setConfirmingId] = useState("");
  const [pendingId, setPendingId] = useState("");

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

  async function handleDelete(agentId: string) {
    setPendingId(agentId);
    setError("");
    setErrorStatus(undefined);
    const response = await deleteAgent(apiClient, agentId);
    setPendingId("");
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    setConfirmingId("");
    setAgents((current) => current.filter((agent) => agent.id !== agentId));
  }

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
          <div className="panel rounded-[1.75rem] p-6" key={agent.id}>
            <Link
              className="block hover:-translate-y-0.5"
              href={`/agents/${agent.id}`}
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
            <div className="mt-5 flex flex-wrap items-center gap-3">
              {confirmingId === agent.id ? (
                <>
                  <button
                    className="rounded-full border border-red-300 bg-red-50 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-red-700 disabled:cursor-not-allowed disabled:opacity-60"
                    disabled={pendingId === agent.id}
                    onClick={() => void handleDelete(agent.id)}
                    type="button"
                  >
                    {pendingId === agent.id ? "Deleting..." : "Confirm Delete"}
                  </button>
                  <button
                    className="rounded-full border border-stone-900/15 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-stone-700 disabled:cursor-not-allowed disabled:opacity-60"
                    disabled={pendingId === agent.id}
                    onClick={() => setConfirmingId("")}
                    type="button"
                  >
                    Cancel
                  </button>
                </>
              ) : (
                <button
                  className="rounded-full border border-red-300 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-red-700 hover:bg-red-50"
                  onClick={() => setConfirmingId(agent.id)}
                  type="button"
                >
                  Delete Agent
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
