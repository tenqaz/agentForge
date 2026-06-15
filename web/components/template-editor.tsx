"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { useApiClient } from "@/components/app-shell";
import {
  apiErrorMessage,
  publishTemplate,
  saveTemplateSoul,
  saveTemplateUser,
  unpublishTemplate,
  updateAdminTemplate,
  type Template,
} from "@/lib/api";

export default function TemplateEditor({
  initialTemplate,
  initialSoul,
  initialUserContent,
  onTemplateChange,
}: {
  initialTemplate: Template;
  initialSoul: string;
  initialUserContent: string;
  onTemplateChange: (template: Template, soul: string, userContent: string) => void;
}) {
  const apiClient = useApiClient();
  const router = useRouter();
  const [template, setTemplate] = useState(initialTemplate);
  const [name, setName] = useState(initialTemplate.name);
  const [description, setDescription] = useState(initialTemplate.description);
  const [soul, setSoul] = useState(initialSoul);
  const [userContent, setUserContent] = useState(initialUserContent);
  const [error, setError] = useState("");
  const [pending, setPending] = useState<string>("");

  function applyTemplate(nextTemplate: Template, nextSoul = soul, nextUser = userContent) {
    setTemplate(nextTemplate);
    setName(nextTemplate.name);
    setDescription(nextTemplate.description);
    onTemplateChange(nextTemplate, nextSoul, nextUser);
    if (nextTemplate.id !== template.id) {
      router.replace(`/admin/templates/${nextTemplate.id}`);
    }
  }

  async function handleMetadataSave() {
    setPending("metadata");
    setError("");
    const response = await updateAdminTemplate(
      apiClient,
      template.id,
      name,
      description,
    );
    setPending("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    applyTemplate(response.data.template);
  }

  async function handleSoulSave() {
    setPending("soul");
    setError("");
    const response = await saveTemplateSoul(apiClient, template.id, soul);
    setPending("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    applyTemplate(response.data.template, soul);
  }

  async function handleUserSave() {
    setPending("user");
    setError("");
    const response = await saveTemplateUser(apiClient, template.id, userContent);
    setPending("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    applyTemplate(response.data.template, soul, userContent);
  }

  async function handlePublishToggle() {
    setPending("publication");
    setError("");
    const response =
      template.status === "published"
        ? await unpublishTemplate(apiClient, template.id)
        : await publishTemplate(apiClient, template.id);
    setPending("");
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    applyTemplate(response.data.template);
  }

  return (
    <div className="grid gap-6">
      <div className="panel rounded-[1.75rem] p-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="eyebrow">Template</p>
            <h2 className="mt-3 text-2xl font-semibold text-stone-950">
              Metadata and publication
            </h2>
          </div>
          <button
            className="rounded-full border border-stone-900/15 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-stone-700 hover:border-stone-900 hover:bg-stone-900 hover:text-stone-50 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={pending === "publication"}
            onClick={() => void handlePublishToggle()}
            type="button"
          >
            {template.status === "published"
              ? pending === "publication"
                ? "Unpublishing..."
                : "Unpublish"
              : pending === "publication"
                ? "Publishing..."
                : "Publish"}
          </button>
        </div>
        <div className="mt-5 grid gap-4">
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
          <button
            className="w-fit rounded-full bg-stone-950 px-5 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)] disabled:cursor-not-allowed disabled:opacity-60"
            disabled={pending === "metadata"}
            onClick={() => void handleMetadataSave()}
            type="button"
          >
            {pending === "metadata" ? "Saving..." : "Save Metadata"}
          </button>
        </div>
      </div>

      <div className="panel rounded-[1.75rem] p-6">
        <p className="eyebrow">SOUL.md</p>
        <textarea
          className="mt-4 min-h-72 w-full rounded-[1.5rem] border border-stone-900/12 bg-white px-4 py-4 font-mono text-sm text-stone-950 shadow-sm"
          onChange={(event) => setSoul(event.target.value)}
          value={soul}
        />
        <button
          className="mt-4 rounded-full bg-stone-950 px-5 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)] disabled:cursor-not-allowed disabled:opacity-60"
          disabled={pending === "soul"}
          onClick={() => void handleSoulSave()}
          type="button"
        >
          {pending === "soul" ? "Saving..." : "Save SOUL"}
        </button>
      </div>

      <div className="panel rounded-[1.75rem] p-6">
        <p className="eyebrow">USER.md</p>
        <textarea
          className="mt-4 min-h-72 w-full rounded-[1.5rem] border border-stone-900/12 bg-white px-4 py-4 font-mono text-sm text-stone-950 shadow-sm"
          onChange={(event) => setUserContent(event.target.value)}
          value={userContent}
        />
        <button
          className="mt-4 rounded-full bg-stone-950 px-5 py-3 text-sm font-semibold uppercase tracking-[0.18em] text-stone-50 hover:bg-[color:var(--accent)] disabled:cursor-not-allowed disabled:opacity-60"
          disabled={pending === "user"}
          onClick={() => void handleUserSave()}
          type="button"
        >
          {pending === "user" ? "Saving..." : "Save USER"}
        </button>
      </div>

      {error ? (
        <div className="rounded-[1.25rem] border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      ) : null}
    </div>
  );
}
