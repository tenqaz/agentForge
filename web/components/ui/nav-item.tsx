"use client";

import Link from "next/link";
import type { ReactNode } from "react";

export default function NavItem({
  href,
  label,
  icon,
  active,
  onClick,
}: {
  href: string;
  label: string;
  icon: ReactNode;
  active: boolean;
  onClick?: () => void;
}) {
  const baseClass =
    "group relative flex items-center gap-3 rounded-[var(--radius-md)] px-3 py-2 text-sm transition";
  const stateClass = active
    ? "bg-[color:var(--color-accent-soft)] text-[color:var(--color-fg)]"
    : "text-[color:var(--color-fg-muted)] hover:bg-[color:var(--color-bg-hover)] hover:text-[color:var(--color-fg)]";

  return (
    <Link
      href={href}
      onClick={onClick}
      aria-current={active ? "page" : undefined}
      className={`${baseClass} ${stateClass}`}
    >
      {active ? (
        <span
          className="absolute left-0 top-1/2 h-5 w-0.5 -translate-y-1/2 rounded-r-full bg-[color:var(--color-accent)]"
          aria-hidden="true"
        />
      ) : null}
      <span
        className={
          active
            ? "text-[color:var(--color-accent)]"
            : "text-[color:var(--color-fg-subtle)] group-hover:text-[color:var(--color-fg-muted)]"
        }
        aria-hidden="true"
      >
        {icon}
      </span>
      <span className="font-medium">{label}</span>
    </Link>
  );
}
