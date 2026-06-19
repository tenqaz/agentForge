import type { HTMLAttributes, ReactNode } from "react";

export type BadgeTone =
  | "neutral"
  | "accent"
  | "success"
  | "warning"
  | "danger"
  | "info";

type BadgeSize = "sm" | "md";

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: BadgeTone;
  size?: BadgeSize;
  children: ReactNode;
};

const toneClass: Record<BadgeTone, string> = {
  neutral:
    "bg-[color:var(--color-bg-hover)] text-[color:var(--color-fg-muted)] border border-[color:var(--color-border-subtle)]",
  accent:
    "bg-[color:var(--color-accent-soft)] text-[color:var(--color-accent)]",
  success:
    "bg-[color:var(--color-success-soft)] text-[color:var(--color-success)]",
  warning:
    "bg-[color:var(--color-warning-soft)] text-[color:var(--color-warning)]",
  danger:
    "bg-[color:var(--color-danger-soft)] text-[color:var(--color-danger)]",
  info: "bg-[color:var(--color-info-soft)] text-[color:var(--color-info)]",
};

const sizeClass: Record<BadgeSize, string> = {
  sm: "h-5 px-2 text-[11px]",
  md: "h-6 px-2.5 text-xs",
};

export default function Badge({
  tone = "neutral",
  size = "sm",
  className = "",
  children,
  ...rest
}: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full font-medium ${toneClass[tone]} ${sizeClass[size]} ${className}`}
      {...rest}
    >
      {children}
    </span>
  );
}
