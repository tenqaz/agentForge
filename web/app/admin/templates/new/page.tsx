"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import { apiErrorMessage, createAdminTemplate } from "@/lib/api";

export default function NewAdminTemplatePage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading, user } = useSessionState();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [pending, setPending] = useState(false);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");

  useEffect(() => {
    if (loading) {
      return;
    }
    if (!user) {
      router.replace("/login");
      return;
    }
    if (user.role !== "admin") {
      router.replace("/templates");
    }
  }, [loading, router, user]);

  async function handleCreate() {
    setPending(true);
    setError("");
    const response = await createAdminTemplate(apiClient, name, description);
    setPending(false);
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    router.push(`/admin/templates/${response.data.template.id}`);
  }

  return (
    <section className="mx-auto max-w-3xl">
      <div className="panel rounded-[2rem] p-8">
        <p className="eyebrow">New Draft</p>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight text-stone-950">
          Create a fresh template draft.
        </h1>
        <div className="mt-6 grid gap-4">
          <label className="grid gap-2 text-sm font-medium text-stone-700">
            Name
            <input
              className="rounded-[1.25rem] border border-stone-900/12 bg-white px-4 py-3 text-base text-stone-950 shadow-sm"
              onChange={(event) => setName(event.target.value)}
              value={name}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium text-stone-700">
            Description
            <textarea
              className="min-h-28 rounded-[1.25rem] border border-stone-900/12 bg-white px-4 py-3 text-base text-stone-950 shadow-sm"
              onChange={(event) => setDescription(event.target.value)}
              value={description}
            />
          </label>
          {error ? (
            <ApiErrorState message={error} status={errorStatus} />
          ) : null}
          <button
            className="w-fit rounded-full bg-stone-950 px-5 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)] disabled:cursor-not-allowed disabled:opacity-60"
            disabled={pending || !name.trim()}
            onClick={() => void handleCreate()}
            type="button"
          >
            {pending ? "Creating..." : "Create Draft"}
          </button>
        </div>
      </div>
    </section>
  );
}
