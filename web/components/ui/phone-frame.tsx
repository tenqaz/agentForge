import type { ReactNode } from "react";

export default function PhoneFrame({ children }: { children?: ReactNode }) {
  return (
    <div className="phone">
      <div className="phone-notch" />
      <div className="phone-screen">
        <div className="phone-status">
          <span>9:41</span>
          <span>AgentForge</span>
        </div>
        {children}
      </div>
    </div>
  );
}
