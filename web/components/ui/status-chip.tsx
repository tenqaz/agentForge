import Badge, { type BadgeTone } from "@/components/ui/badge";

type ChipKind = "agent" | "template" | "channel" | "pairing";

type Entry = { label: string; tone: BadgeTone };

const dictionary: Record<ChipKind, Record<string, Entry>> = {
  agent: {
    creating: { label: "创建中", tone: "warning" },
    provisioning: { label: "配置中", tone: "warning" },
    starting: { label: "启动中", tone: "warning" },
    running: { label: "运行中", tone: "success" },
    stopped: { label: "已停止", tone: "neutral" },
    error: { label: "错误", tone: "danger" },
  },
  template: {
    draft: { label: "草稿", tone: "neutral" },
    published: { label: "已发布", tone: "success" },
    archived: { label: "已归档", tone: "neutral" },
  },
  channel: {
    not_configured: { label: "未配置", tone: "neutral" },
    qr_pending: { label: "等待扫码", tone: "warning" },
    connected: { label: "已连接", tone: "success" },
    error: { label: "错误", tone: "danger" },
    disconnected: { label: "已断开", tone: "neutral" },
  },
  pairing: {
    pending: { label: "等待中", tone: "warning" },
    connected: { label: "已连接", tone: "success" },
    expired: { label: "已过期", tone: "neutral" },
    failed: { label: "失败", tone: "danger" },
  },
};

// BadgeTone → .status-dot 状态类映射
const dotState: Record<BadgeTone, string> = {
  neutral: "idle",
  accent: "pending",
  success: "running",
  warning: "pending",
  danger: "error",
  info: "pending",
};

export default function StatusChip({
  kind,
  value,
}: {
  kind: ChipKind;
  value: string;
  size?: "sm" | "md";
}) {
  const entry = dictionary[kind][value] ?? { label: value, tone: "neutral" as BadgeTone };
  return (
    <Badge tone={entry.tone}>
      <span className={`status-dot ${dotState[entry.tone]}`} aria-hidden="true" />
      {entry.label}
    </Badge>
  );
}

export function statusLabel(kind: ChipKind, value: string): string {
  return dictionary[kind][value]?.label ?? value;
}
