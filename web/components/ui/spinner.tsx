"use client";

type SpinnerSize = "sm" | "md" | "lg";

const sizeClassMap: Record<SpinnerSize, string> = {
  sm: "size-3.5 border",
  md: "size-4 border",
  lg: "size-5 border-2",
};

export default function Spinner({
  size = "md",
  className = "",
}: {
  size?: SpinnerSize;
  className?: string;
}) {
  return (
    <span
      role="status"
      aria-label="加载中"
      className={`inline-block animate-spin rounded-full border-current border-t-transparent ${sizeClassMap[size]} ${className}`}
    />
  );
}
