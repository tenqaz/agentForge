"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { useSessionState } from "@/components/app-shell";

export default function Home() {
  const router = useRouter();
  const { loading, user } = useSessionState();

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

  return (
    <section className="grid gap-6 lg:grid-cols-[1.3fr_0.9fr]">
      <div className="panel rounded-[2rem] p-8 sm:p-12">
        <p className="eyebrow">Operator Console</p>
        <h1 className="mt-6 max-w-3xl text-4xl font-semibold leading-[1.05] tracking-tight text-stone-950 sm:text-6xl">
          Launch agents, publish templates, and complete Weixin pairing without touching a terminal.
        </h1>
        <p className="mt-6 max-w-2xl text-lg leading-8 text-stone-600">
          AgentForge keeps runtime state, template lifecycle, and channel activation in one surface.
          Admins shape templates. Operators launch agents and drive them toward a connected channel.
        </p>
        <div className="mt-10 flex flex-wrap gap-3">
          <Link
            className="rounded-full bg-stone-950 px-6 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)]"
            href="/login"
          >
            Sign In
          </Link>
          <Link
            className="rounded-full border border-stone-900/15 px-6 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-700 hover:border-stone-900 hover:bg-white/70"
            href="/templates"
          >
            Browse Templates
          </Link>
        </div>
      </div>
      <div className="grid gap-6">
        <div className="panel rounded-[2rem] p-8">
          <p className="eyebrow">What Moves Here</p>
          <ul className="mt-5 space-y-4 text-sm leading-7 text-stone-700">
            <li>Published templates for regular operators.</li>
            <li>Admin-only draft editing for SOUL, USER, and whole-skill add/delete.</li>
            <li>Agent runtime progress from creating to running.</li>
            <li>Weixin QR pairing with status refresh and connected-state confirmation.</li>
          </ul>
        </div>
        <div className="rounded-[2rem] border border-[color:var(--border)] bg-stone-950 px-8 py-7 text-stone-50 shadow-[0_24px_80px_rgba(35,28,19,0.18)]">
          <p className="eyebrow text-stone-300">Console Rules</p>
          <p className="mt-5 text-sm leading-7 text-stone-200">
            Secrets stay on the server. Weixin controls stay disabled until runtime is healthy.
            Publication is explicit. Pairing status stays visible without leaking tokens.
          </p>
        </div>
      </div>
    </section>
  );
}
