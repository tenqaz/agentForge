"use client";

import Link from "next/link";
import { Menu } from "lucide-react";
import { forwardRef } from "react";

import type { User } from "@/lib/api";

const MobileTopBar = forwardRef<
  HTMLButtonElement,
  {
    onOpenDrawer: () => void;
    user: User | null;
  }
>(function MobileTopBar({ onOpenDrawer, user }, ref) {
  return (
    <div className="flex h-full w-full items-center justify-between gap-3">
      <button
        ref={ref}
        type="button"
        onClick={onOpenDrawer}
        aria-label="打开导航"
        className="grid size-10 place-items-center rounded-[var(--radius-md)] text-[color:var(--color-fg-muted)] hover:bg-[color:var(--color-bg-hover)] hover:text-[color:var(--color-fg)]"
      >
        <Menu size={18} strokeWidth={1.75} />
      </button>
      <Link href="/" className="flex items-center gap-2 text-sm font-semibold tracking-tight">
        <span className="grid size-6 place-items-center rounded-[var(--radius-sm)] bg-[color:var(--color-accent)] text-[10px] font-semibold text-white">
          AF
        </span>
        AgentForge
      </Link>
      {user ? (
        <span
          className="grid size-8 place-items-center rounded-full bg-[color:var(--color-bg-hover)] text-xs font-semibold uppercase text-[color:var(--color-fg-muted)]"
          aria-hidden="true"
        >
          {user.email.slice(0, 1)}
        </span>
      ) : (
        <Link
          href="/login"
          className="rounded-[var(--radius-md)] px-3 py-1.5 text-xs font-medium text-[color:var(--color-fg-muted)] hover:bg-[color:var(--color-bg-hover)] hover:text-[color:var(--color-fg)]"
        >
          登录
        </Link>
      )}
    </div>
  );
});

export default MobileTopBar;
