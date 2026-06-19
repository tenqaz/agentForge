"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Trash2 } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import AgentRuntimeStatus from "@/components/agent-runtime-status";
import WeixinChannelPanel from "@/components/weixin-channel-panel";
import Button from "@/components/ui/button";
import { Card, CardTitle } from "@/components/ui/card";
import {
  apiErrorMessage,
  deleteAgent,
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
  const { loading: sessionLoading, user } = useSessionState();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [runtime, setRuntime] = useState<AgentRuntime | null>(null);
  const [channel, setChannel] = useState<Channel | null>(null);
  const [session, setSession] = useState<PairingSession | null>(null);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (sessionLoading) {
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
  }, [apiClient, id, sessionLoading, router, user]);

  async function handleDelete() {
    setDeleting(true);
    setError("");
    setErrorStatus(undefined);
    const response = await deleteAgent(apiClient, id);
    if (!response.ok) {
      setDeleting(false);
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      setConfirmingDelete(false);
      return;
    }
    router.push("/agents");
  }

  return (
    <section className="flex flex-col gap-6">
      <Card>
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0">
            <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
              Agent 详情
            </p>
            <CardTitle as="h1" className="mt-2 text-3xl">
              {agent?.name ?? "加载中..."}
            </CardTitle>
            <p className="mt-2 break-all font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
              ID {id}
            </p>
          </div>

          {agent ? (
            <div className="flex flex-wrap items-center gap-2">
              {confirmingDelete ? (
                <>
                  <Button
                    variant="danger"
                    size="sm"
                    disabled={deleting}
                    loading={deleting}
                    onClick={() => void handleDelete()}
                  >
                    {deleting ? "删除中..." : "确认删除"}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={deleting}
                    onClick={() => setConfirmingDelete(false)}
                  >
                    取消
                  </Button>
                </>
              ) : (
                <Button
                  variant="ghost"
                  size="sm"
                  leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
                  onClick={() => setConfirmingDelete(true)}
                  className="text-[color:var(--color-danger)] hover:bg-[color:var(--color-danger-soft)] hover:text-[color:var(--color-danger)]"
                >
                  删除 Agent
                </Button>
              )}
            </div>
          ) : null}
        </div>
      </Card>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

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
