"use client";

import type { ReactNode } from "react";

export type TabItem = {
  key: string;
  label: ReactNode;
  count?: number;
};

export default function Tabs({
  items,
  value,
  onChange,
}: {
  items: TabItem[];
  value: string;
  onChange: (key: string) => void;
}) {
  return (
    <div className="tabs">
      {items.map((item) => (
        <span
          key={item.key}
          className={`tab${item.key === value ? " is-active" : ""}`}
          role="tab"
          aria-selected={item.key === value}
          tabIndex={0}
          onClick={() => onChange(item.key)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              onChange(item.key);
            }
          }}
        >
          {item.label}
          {item.count != null ? <span className="count">{item.count}</span> : null}
        </span>
      ))}
    </div>
  );
}
