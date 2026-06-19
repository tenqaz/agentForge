"use client";

import Image from "next/image";
import { useCallback, useEffect, useState } from "react";
import * as QRCode from "qrcode";

import { useApiClient } from "@/components/app-shell";
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
    if (!session?.qrPayloadUrl) {
      setQrDataURL("");
      return;
    }
    QRCode.toDataURL(session.qrPayloadUrl, { width: 320, margin: 2 })
      .then(setQrDataURL)
      .catch(() => setQrDataURL(""));
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

  return (
    <div className="panel rounded-[1.75rem] p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="eyebrow">Weixin Channel</p>
          <h2 className="mt-3 text-2xl font-semibold text-stone-950">{channel.status}</h2>
          {channel.externalAccountId ? (
            <p className="mt-2 text-sm text-stone-600">
              Connected account: {channel.externalAccountId}
            </p>
          ) : null}
        </div>
        <button
          className="rounded-full bg-stone-950 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-stone-50 hover:bg-[color:var(--accent)] disabled:cursor-not-allowed disabled:opacity-60"
          disabled={disabled || pending}
          onClick={() => void handleCreatePairing()}
          type="button"
        >
          {pending ? "Starting..." : "Create Pairing"}
        </button>
      </div>
      {disabled ? (
        <div className="mt-4 rounded-[1.25rem] border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          Weixin configuration stays disabled until the runtime reaches <code>running</code>.
        </div>
      ) : null}
      {error ? (
        <div className="mt-4 rounded-[1.25rem] border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      ) : null}
      {session ? (
        <div className="mt-5 rounded-[1.5rem] border border-stone-900/10 bg-white/85 p-5">
          <div className="flex items-center justify-between gap-4">
            <p className="text-sm font-semibold uppercase tracking-[0.18em] text-stone-500">
              Pairing Session
            </p>
            <span className="rounded-full border border-stone-900/10 px-3 py-1 text-xs uppercase tracking-[0.16em] text-stone-700">
              {session.status}
            </span>
          </div>
          <p className="mt-3 text-sm text-stone-600">Expires at {session.expiresAt}</p>
          {qrDataURL ? (
            <div className="mt-5 overflow-hidden rounded-[1.5rem] border border-stone-900/10 bg-stone-50 p-4">
              <Image
                alt="Weixin QR code"
                className="mx-auto rounded-[1rem]"
                src={qrDataURL}
                style={{ width: "auto", height: "auto", maxHeight: "18rem" }}
                unoptimized
                width={320}
                height={320}
              />
            </div>
          ) : null}
          {session.qrPayload ? (
            <pre className="mt-4 overflow-x-auto rounded-[1rem] bg-stone-950/95 p-4 font-mono text-xs text-stone-100">
              {session.qrPayload}
            </pre>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
