import type { ReactNode } from "react";

export default function AuthSplit({
  side,
  form,
  admin = false,
}: {
  side: ReactNode;
  form: ReactNode;
  admin?: boolean;
}) {
  return (
    <div className="auth">
      <aside
        className="auth-side"
        style={admin ? { background: "oklch(15% 0.018 250)" } : undefined}
      >
        {side}
      </aside>
      <main className="auth-form-pane">
        <div className="auth-form">{form}</div>
      </main>
    </div>
  );
}
