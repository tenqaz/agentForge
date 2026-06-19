import type { ReactNode } from "react";

export default function EmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-[var(--radius-xl)] border border-dashed border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)]/40 px-6 py-16 text-center">
      {icon ? (
        <div className="text-[color:var(--color-fg-subtle)]" aria-hidden="true">
          {icon}
        </div>
      ) : null}
      <p className="text-base font-medium text-[color:var(--color-fg)]">{title}</p>
      {description ? (
        <p className="max-w-md text-sm text-[color:var(--color-fg-muted)]">{description}</p>
      ) : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}
