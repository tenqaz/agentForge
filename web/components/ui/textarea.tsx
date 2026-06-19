"use client";

import { forwardRef, type TextareaHTMLAttributes } from "react";

type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement> & {
  invalid?: boolean;
  mono?: boolean;
};

const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { invalid = false, mono = false, className = "", ...rest },
  ref,
) {
  const borderColor = invalid
    ? "border-[color:var(--color-danger)]"
    : "border-[color:var(--color-border-default)]";
  const fontClass = mono ? "font-mono text-[13px] leading-relaxed" : "text-sm leading-6";
  return (
    <textarea
      ref={ref}
      aria-invalid={invalid || undefined}
      className={`block w-full rounded-[var(--radius-md)] border ${borderColor} bg-[color:var(--color-bg-input)] px-3 py-2.5 text-[color:var(--color-fg)] placeholder:text-[color:var(--color-fg-subtle)] hover:border-[color:var(--color-border-strong)] focus:border-[color:var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-60 ${fontClass} ${className}`}
      {...rest}
    />
  );
});

export default Textarea;
export type { TextareaProps };
