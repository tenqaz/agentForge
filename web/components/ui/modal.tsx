"use client";

import { useEffect, type ReactNode } from "react";

export default function Modal({
  open,
  onClose,
  title,
  children,
  footer,
}: {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
}) {
  // Esc 关闭 + 打开时锁 body 滚动
  useEffect(() => {
    if (!open) return;
    function handleKey(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }
    window.addEventListener("keydown", handleKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", handleKey);
      document.body.style.overflow = previous;
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="modal-backdrop is-open"
      onClick={(e) => {
        // 点遮罩（非 modal 内容）关闭
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="modal" role="dialog" aria-modal="true">
        {title ? <h3>{title}</h3> : null}
        {children}
        {footer ? <div className="modal-actions">{footer}</div> : null}
      </div>
    </div>
  );
}
