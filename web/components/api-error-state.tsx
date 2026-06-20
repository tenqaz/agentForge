"use client";

import { AlertCircle, AlertTriangle } from "lucide-react";

type ApiErrorStateProps = {
  status?: number;
  message: string;
};

type Tone = "amber" | "red";

function copyForStatus(status?: number): { title: string; tone: Tone; detail: string } {
  switch (status) {
    case 401:
      return {
        title: "需要登录",
        tone: "amber",
        detail: "会话已过期或不存在，请重新登录。",
      };
    case 403:
      return {
        title: "访问被拒绝",
        tone: "amber",
        detail: "当前账户无权限执行该操作。",
      };
    case 409:
      return {
        title: "状态冲突",
        tone: "amber",
        detail: "资源已被其他操作变更，请刷新后重试。",
      };
    default:
      return {
        title: "请求失败",
        tone: "red",
        detail: "控制台未能完成该请求。",
      };
  }
}

export default function ApiErrorState({ status, message }: ApiErrorStateProps) {
  const copy = copyForStatus(status);
  const toneVar = copy.tone === "amber" ? "var(--warning)" : "var(--danger)";
  const toneSoft = copy.tone === "amber" ? "var(--warning-soft)" : "var(--danger-soft)";
  const Icon = copy.tone === "amber" ? AlertTriangle : AlertCircle;

  return (
    <div
      role="alert"
      className="card"
      style={{
        borderColor: `color-mix(in oklch, ${toneVar} 25%, var(--border))`,
        background: toneSoft,
        padding: "14px 16px",
      }}
    >
      <div className="row" style={{ gap: 10, alignItems: "flex-start" }}>
        <span style={{ color: toneVar, marginTop: 2 }} aria-hidden="true">
          <Icon size={16} strokeWidth={1.75} />
        </span>
        <div className="stack-sm" style={{ gap: 4, minWidth: 0, flex: 1 }}>
          <p style={{ fontSize: 14, fontWeight: 600, color: toneVar }}>{copy.title}</p>
          <p style={{ fontSize: 14, lineHeight: 1.6, color: toneVar }}>{copy.detail}</p>
          {message ? (
            <p className="meta" style={{ color: toneVar, wordBreak: "break-word" }}>
              {message}
            </p>
          ) : null}
        </div>
      </div>
    </div>
  );
}
