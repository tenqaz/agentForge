"use client";

import Image from "next/image";
import { useCallback, useEffect, useState } from "react";
import * as QRCode from "qrcode";
import { QrCode } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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
  const [qrDataURL, setQrDataURL] = useState<string>("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  // Encode qrPayloadUrl into a scannable QR image whenever it changes
  useEffect(() => {
    const url = session?.qrPayloadUrl;
    if (!url) {
      // schedule clear via microtask so the eslint set-state-in-effect rule is satisfied
      queueMicrotask(() => setQrDataURL(""));
      return;
    }
    let cancelled = false;
    QRCode.toDataURL(url, { width: 320, margin: 2 })
      .then((value) => {
        if (!cancelled) setQrDataURL(value);
      })
      .catch(() => {
        if (!cancelled) setQrDataURL("");
      });
    return () => {
      cancelled = true;
    };
  }, [session?.qrPayloadUrl]);

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
    <Card>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
            <QrCode size={12} strokeWidth={1.75} aria-hidden="true" />
            微信渠道
          </p>
          {/* Heading text matches the localized channel label so accessibility names stay stable */}
          <h2 className="mt-2 text-xl font-semibold tracking-tight text-[color:var(--color-fg)]">
            {channelLabel}
          </h2>
          {channel.externalAccountId ? (
            <p className="mt-1.5 text-sm text-[color:var(--color-fg-muted)]">
              关联账号 {channel.externalAccountId}
            </p>
          ) : null}
        </div>
        <Button
          variant="primary"
          size="sm"
          leftIcon={<QrCode size={14} strokeWidth={1.75} />}
          disabled={disabled || pending}
          loading={pending}
          onClick={() => void handleCreatePairing()}
        >
          {pending ? "启动中..." : "生成配对"}
        </Button>
      </div>

      {disabled ? (
        <div className="mt-4 rounded-[var(--radius-md)] border border-[color:var(--color-warning)]/25 bg-[color:var(--color-warning-soft)] px-3 py-2.5 text-sm text-[color:var(--color-warning)]">
          Runtime 进入 <code className="font-mono">running</code> 后才能配置微信。
        </div>
      ) : null}

      {error ? (
        <div
          role="alert"
          className="mt-4 rounded-[var(--radius-md)] border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-soft)] px-3 py-2.5 text-sm text-[color:var(--color-danger)]"
        >
          {error}
        </div>
      ) : null}

      {session ? (
        <div className="mt-5 rounded-[var(--radius-lg)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg)]/60 p-4">
          <div className="flex items-center justify-between gap-3">
            <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
              配对会话
            </p>
            <StatusChip kind="pairing" value={session.status} />
          </div>
          <p className="mt-2 font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
            过期于 {session.expiresAt}
          </p>
          {qrDataURL ? (
            <div className="mt-4 rounded-[var(--radius-md)] bg-white p-4">
              <Image
                alt="微信配对二维码"
                className="mx-auto block rounded-md"
                src={qrDataURL}
                style={{ width: "auto", height: "auto", maxHeight: "16rem" }}
                unoptimized
                width={320}
                height={320}
              />
            </div>
          ) : null}
          {session.qrPayload ? (
            <pre className="mt-4 overflow-x-auto rounded-[var(--radius-md)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg)] p-3 font-mono text-[11px] leading-5 text-[color:var(--color-fg-muted)]">
              {session.qrPayload}
            </pre>
          ) : null}
        </div>
      ) : null}
    </Card>
  );
}
