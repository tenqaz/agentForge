"use client";

import Link from "next/link";
import type { ReactNode } from "react";

export default function NavItem({
  href,
  label,
  icon,
  active,
  trail,
  onClick,
}: {
  href: string;
  label: string;
  icon: ReactNode;
  active: boolean;
  trail?: ReactNode;
  onClick?: () => void;
}) {
  return (
    <Link
      href={href}
      onClick={onClick}
      aria-current={active ? "page" : undefined}
      className={`nav-item${active ? " is-active" : ""}`}
    >
      <span className="nav-icon" aria-hidden="true">
        {icon}
      </span>
      <span>{label}</span>
      {trail != null ? <span className="nav-trail">{trail}</span> : null}
    </Link>
  );
}
