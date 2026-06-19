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

const styles = {
  amber: {
    container:
      "rounded-[var(--radius-xl)] border border-[color:var(--color-warning)]/25 bg-[color:var(--color-warning-soft)] px-4 py-3.5",
    title: "text-sm font-semibold text-[color:var(--color-warning)]",
    detail: "text-sm leading-6 text-[color:var(--color-warning)]/90",
    message: "text-xs text-[color:var(--color-warning)]/80",
    Icon: AlertTriangle,
  },
  red: {
    container:
      "rounded-[var(--radius-xl)] border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-soft)] px-4 py-3.5",
    title: "text-sm font-semibold text-[color:var(--color-danger)]",
    detail: "text-sm leading-6 text-[color:var(--color-danger)]/90",
    message: "text-xs text-[color:var(--color-danger)]/80",
    Icon: AlertCircle,
  },
} as const;

export default function ApiErrorState({ status, message }: ApiErrorStateProps) {
  const copy = copyForStatus(status);
  const { container, title, detail, message: messageClass, Icon } = styles[copy.tone];

  return (
    <div role="alert" className={container}>
      <div className="flex items-start gap-2.5">
        <Icon size={16} strokeWidth={1.75} className="mt-0.5 shrink-0" aria-hidden="true" />
        <div className="min-w-0 flex-1">
          <p className={title}>{copy.title}</p>
          <p className={`mt-1 ${detail}`}>{copy.detail}</p>
          {message ? <p className={`mt-2 break-words ${messageClass}`}>{message}</p> : null}
        </div>
      </div>
    </div>
  );
}
