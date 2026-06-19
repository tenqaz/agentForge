"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  useEffect,
  useState,
  useSyncExternalStore,
  type FormEvent,
} from "react";

import { registerWithPassword } from "@/app/register/actions";
import { useApiClient, useSessionState } from "@/components/app-shell";
import { apiErrorMessage } from "@/lib/api";
import Button from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import Input from "@/components/ui/input";

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
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

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

    const response = await registerWithPassword(
      apiClient,
      email.trim(),
      password,
    );
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      setPending(false);
      return;
    }

    setPending(false);
    router.push("/login?registered=1");
    router.refresh();
  }

  return (
    <section className="mx-auto max-w-md py-8 sm:py-12">
      <Card className="p-7 sm:p-8">
        <h1 className="text-2xl font-semibold tracking-tight text-[color:var(--color-fg)]">
          创建控制台账户
        </h1>
        <p className="mt-2 text-sm leading-6 text-[color:var(--color-fg-muted)]">
          使用邮箱和密码注册，然后登录管理你的 Agent 与 Template。
        </p>

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
              autoComplete="new-password"
              placeholder="至少 8 位，包含字母与数字"
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
          >
            {pending ? "创建中..." : "创建账户"}
          </Button>
        </form>

        <p className="mt-6 text-sm text-[color:var(--color-fg-muted)]">
          已有账户？{" "}
          <Link
            href="/login"
            className="font-medium text-[color:var(--color-fg)] underline decoration-[color:var(--color-border-strong)] decoration-1 underline-offset-4 hover:text-[color:var(--color-accent)] hover:decoration-[color:var(--color-accent)]"
          >
            返回登录
          </Link>
        </p>
      </Card>
    </section>
  );
}
