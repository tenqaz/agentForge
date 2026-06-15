"use client";

import { useRouter } from "next/navigation";
import {
  useEffect,
  useState,
  useSyncExternalStore,
  type FormEvent,
} from "react";

import { useApiClient, useSessionState } from "@/components/app-shell";
import { signInWithPassword } from "@/app/login/actions";
import { apiErrorMessage } from "@/lib/api";

export default function LoginPage() {
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
    <section className="mx-auto max-w-2xl">
      <div className="panel rounded-[2rem] p-8 sm:p-10">
        <p className="eyebrow">Session</p>
        <h1 className="mt-5 text-4xl font-semibold tracking-tight text-stone-950">
          Sign in to the console.
        </h1>
        <p className="mt-3 max-w-xl text-base leading-7 text-stone-600">
          Use the same account that owns your agents or the admin account that manages
          template publication.
        </p>
        <form className="mt-8 grid gap-5" onSubmit={(event) => void handleSubmit(event)}>
          <label className="grid gap-2 text-sm font-medium text-stone-700">
            Email
            <input
              className="rounded-[1.25rem] border border-stone-900/12 bg-white px-4 py-3 text-base text-stone-950 shadow-sm"
              name="email"
              onChange={(event) => setEmail(event.target.value)}
              placeholder="user@example.com"
              type="email"
              value={email}
              required
            />
          </label>
          <label className="grid gap-2 text-sm font-medium text-stone-700">
            Password
            <input
              className="rounded-[1.25rem] border border-stone-900/12 bg-white px-4 py-3 text-base text-stone-950 shadow-sm"
              name="password"
              onChange={(event) => setPassword(event.target.value)}
              placeholder="••••••••"
              type="password"
              value={password}
              required
            />
          </label>
          {error ? (
            <div className="rounded-[1.25rem] border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">
              {error}
            </div>
          ) : null}
          <button
            className="mt-2 rounded-full bg-stone-950 px-6 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)] disabled:cursor-not-allowed disabled:opacity-60"
            disabled={!hydrated || pending}
            type="submit"
          >
            {pending ? "Signing In..." : "Sign In"}
          </button>
        </form>
      </div>
    </section>
  );
}
