"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import AgentRuntimeStatus from "@/components/agent-runtime-status";
import WeixinChannelPanel from "@/components/weixin-channel-panel";
import {
  apiErrorMessage,
  getAgent,
  getRuntime,
  getWeixinChannel,
  listWeixinPairingSessions,
  type Agent,
  type AgentRuntime,
  type Channel,
  type PairingSession,
} from "@/lib/api";

export default function AgentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, user } = useSessionState();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [runtime, setRuntime] = useState<AgentRuntime | null>(null);
  const [channel, setChannel] = useState<Channel | null>(null);
  const [session, setSession] = useState<PairingSession | null>(null);
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
      const [agentResponse, runtimeResponse, channelResponse, sessionsResponse] =
        await Promise.all([
          getAgent(apiClient, id),
          getRuntime(apiClient, id),
          getWeixinChannel(apiClient, id),
          listWeixinPairingSessions(apiClient, id),
        ]);

      if (!active) {
        return;
      }
      if (!agentResponse.ok) {
        setErrorStatus(agentResponse.status);
        setError(apiErrorMessage(agentResponse.error.code, agentResponse.error.message));
        return;
      }
      if (!runtimeResponse.ok) {
        setErrorStatus(runtimeResponse.status);
        setError(apiErrorMessage(runtimeResponse.error.code, runtimeResponse.error.message));
        return;
      }
      if (!channelResponse.ok) {
        setErrorStatus(channelResponse.status);
        setError(apiErrorMessage(channelResponse.error.code, channelResponse.error.message));
        return;
      }

      setAgent(agentResponse.data.agent);
      setRuntime(runtimeResponse.data.runtime);
      setChannel(channelResponse.data.channel);
      if (sessionsResponse.ok) {
        setSession(sessionsResponse.data.sessions[0] ?? null);
      }
    })();

    return () => {
      active = false;
    };
  }, [apiClient, id, loading, router, user]);

  return (
    <section className="grid gap-6">
      <div className="panel rounded-[2rem] p-8">
        <p className="eyebrow">Agent Detail</p>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight text-stone-950">
          {agent?.name ?? "Loading agent..."}
        </h1>
        <p className="mt-3 text-base leading-7 text-stone-600">
          Agent ID: {id}
        </p>
      </div>
      {error ? (
        <ApiErrorState message={error} status={errorStatus} />
      ) : null}
      {runtime && channel ? (
        <div className="grid gap-6 lg:grid-cols-2">
          <AgentRuntimeStatus
            agentId={id}
            runtime={runtime}
            onRuntimeChange={setRuntime}
          />
          <WeixinChannelPanel
            agentId={id}
            initialChannel={channel}
            initialSession={session}
            runtimeStatus={runtime.status}
          />
        </div>
      ) : null}
    </section>
  );
}
