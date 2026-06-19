"use client";

import { forwardRef, type InputHTMLAttributes } from "react";

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  invalid?: boolean;
};

const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { invalid = false, className = "", ...rest },
  ref,
) {
  const borderColor = invalid
    ? "border-[color:var(--color-danger)]"
    : "border-[color:var(--color-border-default)]";
  return (
    <input
      ref={ref}
      aria-invalid={invalid || undefined}
      className={`h-10 w-full rounded-[var(--radius-md)] border ${borderColor} bg-[color:var(--color-bg-input)] px-3 text-sm text-[color:var(--color-fg)] placeholder:text-[color:var(--color-fg-subtle)] hover:border-[color:var(--color-border-strong)] focus:border-[color:var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-60 ${className}`}
      {...rest}
    />
  );
});

export default Input;
export type { InputProps };
