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
  const isAdmin = user?.role === "admin";

  return (
    <aside className="sidebar">
      <div className="sidebar-head">
        <Link href="/" className="brand">
          <span className={`brand-mark${isAdmin ? " is-admin" : ""}`}>A</span>
          AgentForge
        </Link>
      </div>

      <nav className="sidebar-nav">
        <NavItem
          href="/templates"
          label="模板浏览"
          icon={<LayoutTemplate size={16} strokeWidth={1.75} />}
          active={pathname === "/templates" || pathname.startsWith("/templates/")}
        />
        <NavItem
          href="/agents"
          label="我的 Agents"
          icon={<Bot size={16} strokeWidth={1.75} />}
          active={pathname === "/agents" || pathname.startsWith("/agents/")}
        />
        {isAdmin ? (
          <>
            <div className="sidebar-section">管理</div>
            <NavItem
              href="/admin/templates"
              label="模板管理"
              icon={<Settings2 size={16} strokeWidth={1.75} />}
              active={pathname.startsWith("/admin")}
            />
          </>
        ) : null}
      </nav>

      <div className="sidebar-foot">
        {loading ? (
          <div className="who">
            <Spinner size="sm" />
            <span className="who-email">加载会话中…</span>
          </div>
        ) : user ? (
          <>
            <span className="avatar" aria-hidden="true">
              {user.email.slice(0, 1).toUpperCase()}
            </span>
            <div className="who">
              <div>{user.email}</div>
              <div className="who-email">{isAdmin ? "管理员" : "普通用户"}</div>
            </div>
            <button
              type="button"
              onClick={() => void onSignOut()}
              aria-label="退出登录"
              className="menu-btn"
            >
              <LogOut size={16} strokeWidth={1.75} />
            </button>
          </>
        ) : (
          <Link href="/login" className="nav-item">
            <span className="nav-icon" aria-hidden="true">
              <LogIn size={16} strokeWidth={1.75} />
            </span>
            <span>登录</span>
          </Link>
        )}
      </div>
    </aside>
  );
}
