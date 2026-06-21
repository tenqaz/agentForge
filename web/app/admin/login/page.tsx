"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, useSyncExternalStore, type FormEvent } from "react";
import { ArrowRight, Check, Lock, Mail } from "lucide-react";

import { useApiClient, useSessionState } from "@/components/app-shell";
import { signInWithPassword } from "@/app/login/actions";
import { apiErrorMessage } from "@/lib/api";
import AuthSplit from "@/components/ui/auth-split";
import Button from "@/components/ui/button";
import Input from "@/components/ui/input";

export default function AdminLoginPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, refreshSession, user } = useSessionState();
  const hydrated = useSyncExternalStore(
    () => () => undefined,
    () => true,
    () => false,
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  useEffect(() => {
    if (loading) return;
    if (user?.role === "admin") {
      router.replace("/admin/templates");
    }
  }, [loading, router, user]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");

    const response = await signInWithPassword(apiClient, email.trim(), password);
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      setPending(false);
      return;
    }

    const signedInUser = await refreshSession();
    setPending(false);
    if (signedInUser?.role !== "admin") {
      setError("此账号不是管理员，请使用普通用户登录。");
      return;
    }
    router.push("/admin/templates");
    router.refresh();
  }

  const side = (
    <>
      <Link href="/" className="brand">
        <span className="brand-mark is-admin">A</span>
        AgentForge{" "}
        <span
          style={{
            fontFamily: "var(--font-mono)",
            fontSize: 11,
            padding: "2px 8px",
            borderRadius: 999,
            background: "var(--accent)",
            color: "var(--surface)",
            marginLeft: 4,
          }}
        >
          ADMIN
        </span>
      </Link>
      <div className="auth-pitch">
        <h2>模板维护后台</h2>
        <p>管理 AgentForge 平台上所有公开模板：人格定义、用户上下文、技能声明、版本发布与归档。</p>
        <ul className="auth-feat-list">
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>SOUL.md / USER.md 编辑</span>
          </li>
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>技能添加 / 删除（MVP）</span>
          </li>
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>版本锁定 · 已创建 Agent 不被影响</span>
          </li>
        </ul>
      </div>
      <div className="auth-foot">© AgentForge · 仅限管理员</div>
    </>
  );

  const form = (
    <>
      <div
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 8,
          padding: "4px 10px",
          background: "var(--accent-soft)",
          color: "var(--accent)",
          borderRadius: 999,
          fontFamily: "var(--font-mono)",
          fontSize: 11,
          letterSpacing: "0.06em",
          textTransform: "uppercase",
          marginBottom: 16,
        }}
      >
        <span style={{ width: 6, height: 6, background: "currentColor", borderRadius: 999 }} />
        管理员入口
      </div>
      <h1>管理员登录</h1>
      <p className="sub">使用平台管理员邮箱登录。</p>

      <form className="form-stack" onSubmit={(event) => void handleSubmit(event)}>
        <div className="field">
          <label className="field-label" htmlFor="admin-email">邮箱</label>
          <div className="input-wrap">
            <span className="input-prefix" aria-hidden="true">
              <Mail size={16} strokeWidth={1.75} />
            </span>
            <Input
              id="admin-email"
              type="email"
              autoComplete="email"
              placeholder="admin@agentforge.dev"
              className="has-prefix"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </div>
        </div>

        <div className="field">
          <label className="field-label" htmlFor="admin-password">密码</label>
          <div className="input-wrap">
            <span className="input-prefix" aria-hidden="true">
              <Lock size={16} strokeWidth={1.75} />
            </span>
            <Input
              id="admin-password"
              type="password"
              autoComplete="current-password"
              placeholder="••••••••"
              className="has-prefix"
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>
        </div>

        {error ? (
          <div
            role="alert"
            className="card"
            style={{
              padding: "10px 12px",
              borderColor: "color-mix(in oklch, var(--danger) 25%, var(--border))",
              background: "var(--danger-soft)",
              color: "var(--danger)",
              fontSize: 13,
            }}
          >
            {error}
          </div>
        ) : null}

        <Button
          type="submit"
          variant="primary"
          size="lg"
          fullWidth
          disabled={!hydrated || pending}
          loading={pending}
          rightIcon={pending ? undefined : <ArrowRight size={16} strokeWidth={1.75} />}
        >
          登录管理后台
        </Button>
      </form>

      <p className="switch-line">
        普通用户登录请前往 <Link href="/login">用户登录</Link>。
      </p>
    </>
  );

  return <AuthSplit side={side} form={form} admin />;
}
