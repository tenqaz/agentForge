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
import { ArrowRight, CheckCircle2 } from "lucide-react";

import { useApiClient, useSessionState } from "@/components/app-shell";
import { signInWithPassword } from "@/app/login/actions";
import { apiErrorMessage } from "@/lib/api";
import Button from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import Input from "@/components/ui/input";

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

    const response = await signInWithPassword(
      apiClient,
      email.trim(),
      password,
    );
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

  return (
    <section className="mx-auto max-w-md py-8 sm:py-12">
      <Card className="p-7 sm:p-8">
        <h1 className="text-2xl font-semibold tracking-tight text-[color:var(--color-fg)]">
          登录控制台
        </h1>
        <p className="mt-2 text-sm leading-6 text-[color:var(--color-fg-muted)]">
          使用拥有 Agent 的账户，或负责 Template 发布的管理员账户。
        </p>

        {registered ? (
          <div
            role="status"
            className="mt-5 flex items-start gap-2.5 rounded-[var(--radius-md)] border border-[color:var(--color-success)]/25 bg-[color:var(--color-success-soft)] px-3.5 py-3 text-sm text-[color:var(--color-success)]"
          >
            <CheckCircle2 size={16} strokeWidth={1.75} className="mt-0.5 shrink-0" aria-hidden="true" />
            <span>账户已创建，请使用新邮箱和密码登录</span>
          </div>
        ) : null}

        <form className="mt-6 grid gap-4" onSubmit={(event) => void handleSubmit(event)}>
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            邮箱
            <Input
              name="email"
              type="email"
              autoComplete="email"
              placeholder="user@example.com"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </label>
          <label className="grid gap-1.5 text-sm font-medium text-[color:var(--color-fg-muted)]">
            密码
            <Input
              name="password"
              type="password"
              autoComplete="current-password"
              placeholder="••••••••"
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>

          {error ? (
            <div
              role="alert"
              className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-soft)] px-3.5 py-2.5 text-sm text-[color:var(--color-danger)]"
            >
              {error}
            </div>
          ) : null}

          <Button
            type="submit"
            variant="primary"
            fullWidth
            disabled={!hydrated || pending}
            loading={pending}
            rightIcon={pending ? undefined : <ArrowRight size={16} strokeWidth={1.75} />}
          >
            {pending ? "登录中..." : "登录"}
          </Button>
        </form>

        <p className="mt-6 text-sm text-[color:var(--color-fg-muted)]">
          还没有账户？{" "}
          <Link
            href="/register"
            className="font-medium text-[color:var(--color-fg)] underline decoration-[color:var(--color-border-strong)] decoration-1 underline-offset-4 hover:text-[color:var(--color-accent)] hover:decoration-[color:var(--color-accent)]"
          >
            创建一个
          </Link>
        </p>
      </Card>
    </section>
  );
}
