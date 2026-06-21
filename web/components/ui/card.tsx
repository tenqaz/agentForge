import type { HTMLAttributes, ReactNode } from "react";

type CardProps = HTMLAttributes<HTMLDivElement> & {
  padded?: boolean;
};

export function Card({ padded = true, className = "", children, ...rest }: CardProps) {
  return (
    <div className={`card${padded ? "" : " card-flat"} ${className}`} {...rest}>
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
    <Tag className={`h3 ${className}`}>
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
    <p className={`muted ${className}`} style={{ marginTop: 6, fontSize: 14, lineHeight: 1.6 }}>
      {children}
    </p>
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
    <div className={`flex flex-wrap items-center gap-3 ${className}`} style={{ marginTop: 20 }} {...rest}>
      {children}
    </div>
  );
}
