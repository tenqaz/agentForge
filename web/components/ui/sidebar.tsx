"use client";

import Link from "next/link";
import { Bot, LayoutTemplate, LogOut, LogIn, Settings2 } from "lucide-react";

import NavItem from "@/components/ui/nav-item";
import Spinner from "@/components/ui/spinner";
import type { User } from "@/lib/api";

export default function Sidebar({
  user,
  loading,
  onSignOut,
  pathname,
}: {
  user: User | null;
  loading: boolean;
  onSignOut: () => void | Promise<void>;
  pathname: string;
}) {
  return (
    <div className="flex h-full flex-col">
      {/* Logo */}
      <div className="flex h-14 items-center gap-2.5 border-b border-[color:var(--color-border-subtle)] px-5">
        <span className="grid size-7 place-items-center rounded-[var(--radius-md)] bg-[color:var(--color-accent)] text-xs font-semibold text-white">
          AF
        </span>
        <Link
          href="/"
          className="text-sm font-semibold tracking-tight text-[color:var(--color-fg)]"
        >
          AgentForge
        </Link>
      </div>

      {/* Nav items */}
      <nav className="flex-1 px-3 py-4">
        <p className="px-3 pb-2 text-[11px] font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
          工作区
        </p>
        <div className="flex flex-col gap-0.5">
          <NavItem
            href="/templates"
            label="Templates"
            icon={<LayoutTemplate size={16} strokeWidth={1.75} />}
            active={pathname === "/templates" || pathname.startsWith("/templates/")}
          />
          <NavItem
            href="/agents"
            label="Agents"
            icon={<Bot size={16} strokeWidth={1.75} />}
            active={pathname === "/agents" || pathname.startsWith("/agents/")}
          />
          {user?.role === "admin" ? (
            <NavItem
              href="/admin/templates"
              label="管理"
              icon={<Settings2 size={16} strokeWidth={1.75} />}
              active={pathname.startsWith("/admin")}
            />
          ) : null}
        </div>
      </nav>

      {/* User card */}
      <div className="border-t border-[color:var(--color-border-subtle)] p-3">
        {loading ? (
          <div className="flex items-center gap-2 px-2 py-1.5 text-xs text-[color:var(--color-fg-muted)]">
            <Spinner size="sm" />
            <span>加载会话中...</span>
          </div>
        ) : user ? (
          <div className="flex items-center gap-3 rounded-[var(--radius-md)] px-2 py-2">
            <span
              className="grid size-8 shrink-0 place-items-center rounded-full bg-[color:var(--color-bg-hover)] text-xs font-semibold uppercase text-[color:var(--color-fg-muted)]"
              aria-hidden="true"
            >
              {user.email.slice(0, 1)}
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-[color:var(--color-fg)]">{user.email}</p>
              <p className="text-[11px] uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
                {user.role === "admin" ? "管理员" : "普通用户"}
              </p>
            </div>
            <button
              type="button"
              onClick={() => void onSignOut()}
              aria-label="退出登录"
              className="grid size-8 shrink-0 place-items-center rounded-[var(--radius-md)] text-[color:var(--color-fg-muted)] hover:bg-[color:var(--color-bg-hover)] hover:text-[color:var(--color-fg)]"
            >
              <LogOut size={16} strokeWidth={1.75} />
            </button>
          </div>
        ) : (
          <Link
            href="/login"
            className="flex items-center gap-2 rounded-[var(--radius-md)] px-3 py-2 text-sm text-[color:var(--color-fg-muted)] hover:bg-[color:var(--color-bg-hover)] hover:text-[color:var(--color-fg)]"
          >
            <LogIn size={16} strokeWidth={1.75} />
            <span>登录</span>
          </Link>
        )}
      </div>
    </div>
  );
}
