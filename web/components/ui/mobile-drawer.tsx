"use client";

import { X } from "lucide-react";
import { useEffect, useRef } from "react";

import Sidebar from "@/components/ui/sidebar";
import type { User } from "@/lib/api";

export default function MobileDrawer({
  open,
  onClose,
  user,
  loading,
  onSignOut,
  pathname,
}: {
  open: boolean;
  onClose: () => void;
  user: User | null;
  loading: boolean;
  onSignOut: () => void | Promise<void>;
  pathname: string;
}) {
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);

  // Lock body scroll while open
  useEffect(() => {
    if (!open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previous;
    };
  }, [open]);

  // Close on Esc; focus close button on open
  useEffect(() => {
    if (!open) return;
    closeButtonRef.current?.focus();
    function handleKey(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [open, onClose]);

  if (!open) {
    return null;
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="导航"
      className="fixed inset-0 z-50 lg:hidden"
    >
      <div
        className="absolute inset-0"
        style={{ background: "color-mix(in oklch, var(--fg) 50%, transparent)" }}
        onClick={onClose}
        aria-hidden="true"
      />
      <div className="absolute inset-y-0 left-0 flex w-[280px] max-w-[85%] flex-col shadow-2xl">
        <button
          ref={closeButtonRef}
          type="button"
          onClick={onClose}
          aria-label="关闭导航"
          className="btn btn-icon btn-ghost absolute right-3 top-3 z-10"
        >
          <X size={16} strokeWidth={1.75} />
        </button>
        <Sidebar
          user={user}
          loading={loading}
          onSignOut={onSignOut}
          pathname={pathname}
        />
      </div>
    </div>
  );
}
