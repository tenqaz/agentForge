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
    <section className="mx-auto max-w-2xl">
      <div className="panel rounded-[2rem] p-8 sm:p-10">
        <p className="eyebrow">Account</p>
        <h1 className="mt-5 text-4xl font-semibold tracking-tight text-stone-950">
          Create your console account.
        </h1>
        <p className="mt-3 max-w-xl text-base leading-7 text-stone-600">
          Register with an email and password, then sign in to manage your
          agents and templates.
        </p>
        <form className="mt-8 grid gap-5" onSubmit={(event) => void handleSubmit(event)}>
          <label className="grid gap-2 text-sm font-medium text-stone-700">
            Email
            <input
              className="rounded-[1.25rem] border border-stone-900/12 bg-white px-4 py-3 text-base text-stone-950 shadow-sm"
              name="email"
              onChange={(event) => setEmail(event.target.value)}
              placeholder="user@example.com"
              required
              type="email"
              value={email}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium text-stone-700">
            Password
            <input
              className="rounded-[1.25rem] border border-stone-900/12 bg-white px-4 py-3 text-base text-stone-950 shadow-sm"
              name="password"
              onChange={(event) => setPassword(event.target.value)}
              placeholder="At least 8 characters with letters and numbers"
              required
              type="password"
              value={password}
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
            {pending ? "Creating Account..." : "Create Account"}
          </button>
        </form>
        <p className="mt-6 text-sm text-stone-600">
          Already have an account?{" "}
          <Link className="font-semibold text-stone-950 underline decoration-stone-300 underline-offset-4" href="/login">
            Back to sign in
          </Link>
        </p>
      </div>
    </section>
  );
}
