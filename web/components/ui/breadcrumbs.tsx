import Link from "next/link";

export type Crumb = {
  label: string;
  href?: string;
};

export default function Breadcrumbs({ items }: { items: Crumb[] }) {
  return (
    <div className="topbar-trail">
      {items.map((c, i) => (
        <span key={i} style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
          {c.href ? (
            <Link href={c.href} style={{ color: "var(--muted)" }}>
              {c.label}
            </Link>
          ) : (
            <span className="crumb-active">{c.label}</span>
          )}
          {i < items.length - 1 ? <span className="sep">/</span> : null}
        </span>
      ))}
    </div>
  );
}
