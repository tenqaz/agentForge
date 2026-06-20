"use client";

import { useCallback, useEffect, useState } from "react";
import { QrCode } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
import QRBox, { type QRState } from "@/components/ui/qr-box";
import StatusChip, { statusLabel } from "@/components/ui/status-chip";
import {
  apiErrorMessage,
  createWeixinPairingSession,
  ensureWeixinChannel,
  getWeixinChannel,
  getWeixinPairingSession,
  type Channel,
  type PairingSession,
} from "@/lib/api";

function sessionToQrState(status: PairingSession["status"]): QRState {
  switch (status) {
    case "connected":
      return "confirmed";
    case "expired":
    case "failed":
      return "expired";
    default:
      return "pending";
  }
}

export default function WeixinChannelPanel({
  agentId,
  initialChannel,
  initialSession,
  runtimeStatus,
}: {
  agentId: string;
  initialChannel: Channel;
  initialSession: PairingSession | null;
  runtimeStatus: string;
}) {
  const apiClient = useApiClient();
  const [channel, setChannel] = useState(initialChannel);
  const [session, setSession] = useState<PairingSession | null>(initialSession);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    if (!session?.id) {
      const channelResponse = await getWeixinChannel(apiClient, agentId);
      if (channelResponse.ok) {
        setChannel(channelResponse.data.channel);
      }
      return;
    }

    const sessionResponse = await getWeixinPairingSession(apiClient, agentId, session.id);
    if (sessionResponse.ok) {
      setSession(sessionResponse.data.session);
    }

    const channelResponse = await getWeixinChannel(apiClient, agentId);
    if (channelResponse.ok) {
      setChannel(channelResponse.data.channel);
    }
  }, [agentId, apiClient, session]);

  useEffect(() => {
    if (runtimeStatus !== "running") {
      return;
    }
    if (!session || session.status !== "pending") {
      return;
    }
    const timer = window.setInterval(() => {
      void refresh();
    }, 2000);
    return () => window.clearInterval(timer);
  }, [refresh, runtimeStatus, session]);

  async function handleCreatePairing() {
    setPending(true);
    setError("");

    const ensureResponse = await ensureWeixinChannel(apiClient, agentId);
    if (!ensureResponse.ok) {
      setError(apiErrorMessage(ensureResponse.error.code, ensureResponse.error.message));
      setPending(false);
      return;
    }
    setChannel(ensureResponse.data.channel);

    const pairingResponse = await createWeixinPairingSession(apiClient, agentId);
    setPending(false);
    if (!pairingResponse.ok) {
      setError(apiErrorMessage(pairingResponse.error.code, pairingResponse.error.message));
      return;
    }
    setSession(pairingResponse.data.session);
    await refresh();
  }

  const disabled = runtimeStatus !== "running";
  const channelLabel = statusLabel("channel", channel.status);

  return (
    <section className="section-card">
      <div className="section-card-head">
        <div className="row" style={{ gap: 10, alignItems: "flex-start", minWidth: 0 }}>
          <span style={{ color: "var(--accent)", marginTop: 2 }} aria-hidden="true">
            <QrCode size={16} strokeWidth={1.75} />
          </span>
          <div style={{ minWidth: 0 }}>
            <h3 style={{ fontSize: 15, fontWeight: 600 }}>{channelLabel}</h3>
            {channel.externalAccountId ? (
              <span className="meta">关联账号 {channel.externalAccountId}</span>
            ) : null}
          </div>
        </div>
        <Button
          variant="primary"
          size="sm"
          leftIcon={<QrCode size={14} strokeWidth={1.75} />}
          disabled={disabled || pending}
          loading={pending}
          onClick={() => void handleCreatePairing()}
        >
          {pending ? "启动中…" : "生成配对"}
        </Button>
      </div>

      <div className="section-card-body">
        {disabled ? (
          <div
            className="card"
            style={{
              padding: "10px 12px",
              borderColor: "color-mix(in oklch, var(--warning) 25%, var(--border))",
              background: "var(--warning-soft)",
              color: "oklch(50% 0.14 80)",
              fontSize: 13,
            }}
          >
            Runtime 进入 <code className="num">running</code> 后才能配置微信。
          </div>
        ) : null}

        {error ? (
          <div
            role="alert"
            className="card"
            style={{
              marginTop: 12,
              padding: "10px 12px",
              borderColor: "color-mix(in oklch, var(--danger) 25%, var(--border))",
              background: "var(--danger-soft)",
              color: "var(--danger)",
              fontSize: 13,
            }}
          >
            {error}
          </div>
        ) : null}

        {session ? (
          <div className="stack" style={{ marginTop: 16, gap: 14 }}>
            <div className="row-between">
              <span className="meta">配对会话</span>
              <StatusChip kind="pairing" value={session.status} />
            </div>
            <span className="meta">过期于 {session.expiresAt}</span>
            {session.qrPayloadUrl ? (
              <QRBox payload={session.qrPayloadUrl} state={sessionToQrState(session.status)} />
            ) : null}
            {session.qrPayload ? (
              <pre
                className="num"
                style={{
                  marginTop: 4,
                  overflowX: "auto",
                  padding: 12,
                  border: "1px solid var(--border)",
                  borderRadius: "var(--radius)",
                  background: "var(--surface-2)",
                  fontSize: 11,
                  lineHeight: 1.5,
                  color: "var(--muted)",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-all",
                }}
              >
                {session.qrPayload}
              </pre>
            ) : null}
          </div>
        ) : null}
      </div>
    </section>
  );
}
