import type { CSSProperties } from "react";

export default function Skeleton({
  className = "",
  style,
}: {
  className?: string;
  style?: CSSProperties;
}) {
  return <div className={`sk ${className}`} style={style} />;
}
