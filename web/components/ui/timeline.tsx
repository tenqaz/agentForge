import type { ReactNode } from "react";

export type TimelineItemState = "pending" | "active" | "done" | "error";

export type TimelineItem = {
  title: string;
  sub?: string;
  time?: string;
  state: TimelineItemState;
  marker?: ReactNode;
};

const markerClass: Record<TimelineItemState, string> = {
  pending: "",
  active: "is-active",
  done: "is-done",
  error: "is-error",
};

export default function Timeline({ items }: { items: TimelineItem[] }) {
  return (
    <div className="timeline">
      {items.map((item, idx) => (
        <div className="tl-item" key={idx}>
          <span className={`tl-marker ${markerClass[item.state]}`.trim()}>
            {item.marker ?? (item.state === "done" ? "✓" : idx + 1)}
          </span>
          <div>
            <div className="tl-title">{item.title}</div>
            {item.sub ? <div className="tl-sub">{item.sub}</div> : null}
          </div>
          <span className="tl-time">{item.time ?? "— —"}</span>
        </div>
      ))}
    </div>
  );
}
