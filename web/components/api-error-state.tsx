"use client";

type ApiErrorStateProps = {
  status?: number;
  message: string;
};

function copyForStatus(status?: number) {
  switch (status) {
    case 401:
      return {
        title: "Sign-in required",
        tone: "amber",
        detail: "Your session is missing or expired. Sign in again to continue.",
      };
    case 403:
      return {
        title: "Access denied",
        tone: "amber",
        detail: "This action is blocked for the current account.",
      };
    case 409:
      return {
        title: "State conflict",
        tone: "amber",
        detail: "The resource changed underneath this action. Refresh and try again.",
      };
    default:
      return {
        title: "Request failed",
        tone: "red",
        detail: "The console could not finish this request.",
      };
  }
}

export default function ApiErrorState({ status, message }: ApiErrorStateProps) {
  const copy = copyForStatus(status);
  const className =
    copy.tone === "amber"
      ? "rounded-[1.5rem] border border-amber-300 bg-amber-50 px-5 py-4 text-amber-900"
      : "rounded-[1.5rem] border border-red-300 bg-red-50 px-5 py-4 text-red-700";

  return (
    <div className={className}>
      <p className="text-sm font-semibold uppercase tracking-[0.16em]">{copy.title}</p>
      <p className="mt-2 text-sm">{copy.detail}</p>
      <p className="mt-3 text-sm">{message}</p>
    </div>
  );
}
