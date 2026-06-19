import type { HTMLAttributes, ReactNode } from "react";

type CardProps = HTMLAttributes<HTMLDivElement> & {
  padded?: boolean;
};

export function Card({ padded = true, className = "", children, ...rest }: CardProps) {
  return (
    <div
      className={`rounded-[var(--radius-xl)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)] ${padded ? "p-6" : ""} ${className}`}
      {...rest}
    >
      {children}
    </div>
  );
}

export function CardHeader({ className = "", children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={`flex flex-wrap items-start justify-between gap-4 ${className}`} {...rest}>
      {children}
    </div>
  );
}

export function CardTitle({
  as: Tag = "h2",
  className = "",
  children,
}: {
  as?: "h1" | "h2" | "h3";
  className?: string;
  children: ReactNode;
}) {
  return (
    <Tag className={`text-xl font-semibold tracking-tight text-[color:var(--color-fg)] ${className}`}>
      {children}
    </Tag>
  );
}

export function CardDescription({
  className = "",
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <p className={`mt-1.5 text-sm leading-6 text-[color:var(--color-fg-muted)] ${className}`}>{children}</p>
  );
}

export function CardContent({ className = "", children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={className} {...rest}>
      {children}
    </div>
  );
}

export function CardFooter({ className = "", children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={`mt-5 flex flex-wrap items-center gap-3 ${className}`} {...rest}>
      {children}
    </div>
  );
}
