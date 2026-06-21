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
    <div className="flex h-full w-full items-center justify-between gap-3 lg:hidden">
      <button
        ref={ref}
        type="button"
        onClick={onOpenDrawer}
        aria-label="打开导航"
        className="btn btn-icon btn-ghost"
      >
        <Menu size={18} strokeWidth={1.75} />
      </button>
      <Link href="/" className="brand">
        <span className="brand-mark">A</span>
        AgentForge
      </Link>
      {user ? (
        <span className="avatar" aria-hidden="true">
          {user.email.slice(0, 1).toUpperCase()}
        </span>
      ) : (
        <Link href="/login" className="btn btn-ghost btn-sm">
          登录
        </Link>
      )}
    </div>
  );
});

export default MobileTopBar;
