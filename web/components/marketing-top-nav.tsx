"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { useApiClient, useSessionState } from "@/components/app-shell";

// 营销首页顶部导航：视觉沿用 marketing.css 中 .topnav / .btn 系列，
// 交互态由 SessionContext 驱动。未登录态用于 SSR 首屏（与未水合时一致），
// 登录态在客户端 hydrate 后切换，避免闪烁与 hydration mismatch。
export default function MarketingTopNav() {
  const { loading, user, clearSession } = useSessionState();
  const apiClient = useApiClient();
  const router = useRouter();

  async function handleSignOut() {
    await apiClient.delete("/api/session");
    clearSession();
    router.refresh();
  }

  return (
    <header className="topnav" data-od-id="topnav">
      <div className="container topnav-inner">
        <span className="logo">
          <span className="logo-mark">A</span>
          AgentForge
        </span>
        <nav>
          <a href="#features">功能</a>
          <a href="#flow">三步流程</a>
          <a href="#cases">使用场景</a>
          <a href="#wechat">微信接入</a>
          <a href="#roadmap">里程碑</a>
        </nav>
        <div className="row" style={{ gap: 8 }}>
          {/*
            loading 期间维持「未登录态」骨架而不是空，可让 SSR/水合期间
            视觉稳定；待 refreshSession 完成后再切换到登录态。
          */}
          {!loading && user ? (
            <>
              <Link className="btn btn-ghost" href="/agents">
                进入控制台
              </Link>
              <button
                type="button"
                className="btn btn-primary"
                onClick={handleSignOut}
              >
                退出
              </button>
            </>
          ) : (
            <Link className="btn btn-ghost" href="/login">
              登录
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
