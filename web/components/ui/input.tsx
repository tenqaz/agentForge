"use client";

import { forwardRef, type InputHTMLAttributes } from "react";

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  invalid?: boolean;
};

const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { invalid = false, className = "", style, ...rest },
  ref,
) {
  return (
    <input
      ref={ref}
      aria-invalid={invalid || undefined}
      className={`input ${className}`}
      style={invalid ? { borderColor: "var(--danger)", ...(style ?? {}) } : style}
      {...rest}
    />
  );
});

export default Input;
export type { InputProps };
