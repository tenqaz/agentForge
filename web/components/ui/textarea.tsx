"use client";

import { forwardRef, type TextareaHTMLAttributes } from "react";

type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement> & {
  invalid?: boolean;
  mono?: boolean;
};

const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { invalid = false, mono = false, className = "", style, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      aria-invalid={invalid || undefined}
      className={`textarea${mono ? " textarea-mono" : ""} ${className}`}
      style={invalid ? { borderColor: "var(--danger)", ...(style ?? {}) } : style}
      {...rest}
    />
  );
});

export default Textarea;
export type { TextareaProps };
