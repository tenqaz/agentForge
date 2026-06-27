"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  useEffect,
  useState,
  useSyncExternalStore,
  type FormEvent,
} from "react";
import { ArrowRight, Check, Lock, Mail } from "lucide-react";

import { registerWithPassword, sendRegisterEmailCode } from "@/app/register/actions";
import { useApiClient, useSessionState } from "@/components/app-shell";
import { apiErrorMessage } from "@/lib/api";
import Button from "@/components/ui/button";
import Input from "@/components/ui/input";
import AuthSplit from "@/components/ui/auth-split";

export default function RegisterPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, user } = useSessionState();
  const hydrated = useSyncExternalStore(
    () => () => undefined,
    () => true,
    () => false,
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [email, setEmail] = useState("");
  const [emailCode, setEmailCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [sendingCode, setSendingCode] = useState(false);
  const [cooldownSeconds, setCooldownSeconds] = useState(0);

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

  useEffect(() => {
    if (cooldownSeconds <= 0) return;
    const timer = window.setTimeout(() => {
      setCooldownSeconds((value) => value - 1);
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [cooldownSeconds]);

  async function handleSendCode() {
    setError("");
    setNotice("");
    setSendingCode(true);
    try {
      const response = await sendRegisterEmailCode(apiClient, email.trim());
      if (!response.ok) {
        const retryAfter = Number(response.headers.get("retry-after") ?? "0");
        if (retryAfter > 0) {
          setCooldownSeconds(retryAfter);
        }
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      // 与后端 verification.CooldownWindow (60s) 保持一致。
      setCooldownSeconds(60);
      setNotice("验证码已发送，请检查邮箱。");
    } catch {
      setError("网络异常，请检查连接后重试。");
    } finally {
      setSendingCode(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (password !== confirmPassword) {
      setError("两次输入的密码不一致");
      return;
    }
    setPending(true);
    setError("");

    try {
      const response = await registerWithPassword(apiClient, email.trim(), password, emailCode.trim());
      if (!response.ok) {
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }

      router.push("/login?registered=1");
      router.refresh();
    } catch {
      setError("网络异常，请检查连接后重试。");
    } finally {
      setPending(false);
    }
  }

  const side = (
    <>
      <Link href="/" className="brand">
        <span className="brand-mark">A</span>
        AgentForge
      </Link>
      <div className="auth-pitch">
        <h2>把 Agent 工程化里那些恶心的东西，全部交给平台。</h2>
        <p>注册一个账号，就能从模板创建你的第一个 Agent。我们处理运行时、二维码、断线重连、凭据加密——你专注 Agent 的人格与能力。</p>
        <ul className="auth-feat-list">
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>注册即送一个示范模板</span>
          </li>
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>支持并行托管多个 Agent</span>
          </li>
          <li>
            <Check size={18} strokeWidth={1.75} />
            <span>密钥仅在服务端 · 不出现在前端</span>
          </li>
        </ul>
      </div>
      <div className="auth-foot">© AgentForge · 2026</div>
    </>
  );

  const form = (
    <>
      <h1>创建账号</h1>
      <p className="sub">使用邮箱注册一个 AgentForge 账号。</p>

      <form className="form-stack" onSubmit={(event) => void handleSubmit(event)}>
        <div className="field">
          <label className="field-label" htmlFor="register-email">邮箱</label>
          <div className="input-wrap">
            <span className="input-prefix" aria-hidden="true">
              <Mail size={16} strokeWidth={1.75} />
            </span>
            <Input
              id="register-email"
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
          <label className="field-label" htmlFor="register-email-code">验证码</label>
          <div className="input-wrap" style={{ display: "flex", gap: 8 }}>
            <Input
              id="register-email-code"
              name="emailCode"
              type="text"
              inputMode="numeric"
              autoComplete="one-time-code"
              placeholder="6 位验证码"
              required
              value={emailCode}
              onChange={(event) => setEmailCode(event.target.value)}
              style={{ flex: 1 }}
            />
            <Button
              type="button"
              variant="secondary"
              disabled={!hydrated || sendingCode || cooldownSeconds > 0}
              onClick={() => void handleSendCode()}
            >
              {sendingCode
                ? "发送中…"
                : cooldownSeconds > 0
                  ? `${cooldownSeconds} 秒后重发`
                  : "发送验证码"}
            </Button>
          </div>
        </div>

        <div className="field">
          <label className="field-label" htmlFor="register-password">密码</label>
          <div className="input-wrap">
            <span className="input-prefix" aria-hidden="true">
              <Lock size={16} strokeWidth={1.75} />
            </span>
            <Input
              id="register-password"
              name="password"
              type="password"
              autoComplete="new-password"
              placeholder="至少 8 位，包含字母与数字"
              className="has-prefix"
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>
        </div>

        <div className="field">
          <label className="field-label" htmlFor="register-confirm-password">确认密码</label>
          <div className="input-wrap">
            <span className="input-prefix" aria-hidden="true">
              <Lock size={16} strokeWidth={1.75} />
            </span>
            <Input
              id="register-confirm-password"
              name="confirmPassword"
              type="password"
              autoComplete="new-password"
              placeholder="再次输入密码"
              className="has-prefix"
              required
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
            />
          </div>
        </div>

        {notice ? (
          <div
            role="status"
            className="card"
            style={{
              padding: "10px 12px",
              fontSize: 13,
            }}
          >
            {notice}
          </div>
        ) : null}

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
          {pending ? "创建中…" : "创建账户"}
        </Button>
      </form>

      <p className="switch-line">
        已有账号？<Link href="/login">直接登录</Link>
      </p>
    </>
  );

  return <AuthSplit side={side} form={form} />;
}
