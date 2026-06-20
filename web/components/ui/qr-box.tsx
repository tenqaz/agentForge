"use client";

import { useEffect, useState } from "react";
import * as QRCode from "qrcode";

export type QRState = "pending" | "scanned" | "confirmed" | "expired";

export default function QRBox({
  payload,
  state,
  onRefresh,
}: {
  payload: string;
  state: QRState;
  onRefresh?: () => void;
}) {
  const [svg, setSvg] = useState<string>("");

  useEffect(() => {
    if (!payload) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSvg("");
      return;
    }
    let cancelled = false;
    QRCode.toString(
      payload,
      { type: "svg", margin: 0, errorCorrectionLevel: "M" },
      (err, result) => {
        if (cancelled) return;
        if (err) {
          setSvg("");
          return;
        }
        setSvg(result);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [payload]);

  const stateClass = [
    state === "scanned" || state === "confirmed" ? "scanned" : "",
    state === "confirmed" ? "is-confirmed" : "",
    state === "expired" ? "is-expired" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      className={`qr-box ${stateClass}`.trim()}
      onClick={state === "expired" && onRefresh ? onRefresh : undefined}
      role={state === "expired" && onRefresh ? "button" : undefined}
      tabIndex={state === "expired" && onRefresh ? 0 : undefined}
    >
      {svg ? (
        <span aria-label="二维码" dangerouslySetInnerHTML={{ __html: svg }} />
      ) : null}
      <div className="qr-overlay">
        {state === "confirmed" ? <div className="qr-tick">✓</div> : null}
      </div>
    </div>
  );
}
