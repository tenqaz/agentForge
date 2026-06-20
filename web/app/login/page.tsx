"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Suspense,
  useEffect,
  useState,
  useSyncExternalStore,
  type FormEvent,
} from "react";
import { ArrowRight, Check, Lock, Mail } from "lucide-react";

import { useApiClient, useSessionState } from "@/components/app-shell";
import { signInWithPassword } from "@/app/login/actions";
import { apiErrorMessage } from "@/lib/api";
import Button from "@/components/ui/button";
import Input from "@/components/ui/input";
import AuthSplit from "@/components/ui/auth-split";

export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <LoginPageInner />
    </Suspense>
  );
}

function LoginPageInner() {
  const apiClient = useApiClient();
  const router = useRouter();
  const searchParams = useSearchParams();
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
  const registered = searchParams.get("registered") === "1";

  useEffect(() => {
    if (loading) {
      return;
    }
    if (user?.role === "admin") {
      router.replace("/admin/templates");
      return;
    }
    if (user) {
      router.replace("/templates");
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
    if (signedInUser?.role === "admin") {
      router.push("/admin/templates");
      router.refresh();
      return;
    }
    router.push("/templates");
    router.refresh();
  }

  const side = (
    <>
      <Link href="/" className="brand">
        <span className="brand-mark">A</span>
        AgentForge
      </Link>
      <div className="auth-pitch">
        <h2>
          一份模板，
          <br />
          一个活在你微信里的 AI Agent。
        </h2>
        <p>从模板出发、几次点击拥有一个独立运行的 Agent；扫一次微信码，它就活在你的对话列表里——不写代码、不开服务器。</p>
        <ul className="auth-feat-list">
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>每个 Agent 独立运行时与数据目录</span>
          </li>
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>微信扫码全流程托管 · 凭据加密落盘</span>
          </li>
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>多用户隔离 · 数据从不互通</span>
          </li>
        </ul>
      </div>
      <div className="auth-foot">© AgentForge · 2026</div>
    </>
  );

  const form = (
    <>
      <h1>欢迎回来</h1>
      <p className="sub">使用邮箱与密码登录你的 AgentForge 账号。</p>

      {registered ? (
        <div
          role="status"
          className="card"
          style={{
            marginBottom: 16,
            padding: "10px 12px",
            borderColor: "color-mix(in oklch, var(--success) 25%, var(--border))",
            background: "var(--success-soft)",
            color: "var(--success)",
            fontSize: 13,
            display: "flex",
            gap: 8,
            alignItems: "flex-start",
          }}
        >
          <Check size={16} strokeWidth={1.75} style={{ marginTop: 2 }} aria-hidden="true" />
          <span>账户已创建，请使用新邮箱和密码登录</span>
        </div>
      ) : null}

      <form className="form-stack" onSubmit={(event) => void handleSubmit(event)}>
        <div className="field">
          <label className="field-label" htmlFor="login-email">邮箱</label>
          <div className="input-wrap">
            <span className="input-prefix" aria-hidden="true">
              <Mail size={16} strokeWidth={1.75} />
            </span>
            <Input
              id="login-email"
              name="email"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              className="has-prefix"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </div>
        </div>

        <div className="field">
          <label className="field-label" htmlFor="login-password">密码</label>
          <div className="input-wrap">
            <span className="input-prefix" aria-hidden="true">
              <Lock size={16} strokeWidth={1.75} />
            </span>
            <Input
              id="login-password"
              name="password"
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
          {pending ? "登录中…" : "登录"}
        </Button>
      </form>

      <p className="switch-line">
        还没有账号？<Link href="/register">免费注册</Link>
      </p>
    </>
  );

  return <AuthSplit side={side} form={form} />;
}
