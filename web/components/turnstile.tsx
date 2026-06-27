"use client";

import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import Script from "next/script";

declare global {
  interface Window {
    turnstile?: {
      render: (container: string | HTMLElement, options: Record<string, unknown>) => string;
      reset: (id?: string) => void;
      remove: (id: string) => void;
    };
  }
}

const TURNSTILE_SCRIPT = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";

export type TurnstileHandle = {
  reset: () => void;
};

export type TurnstileProps = {
  sitekey: string;
  action: string;
  onToken: (token: string) => void;
  onExpire?: () => void;
  onError?: (code: string) => void;
  theme?: "auto" | "light" | "dark";
  className?: string;
};

export const Turnstile = forwardRef<TurnstileHandle, TurnstileProps>(function Turnstile(
  { sitekey, action, onToken, onExpire, onError, theme = "auto", className },
  ref,
) {
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | null>(null);
  const [scriptReady, setScriptReady] = useState(false);

  useImperativeHandle(ref, () => ({
    reset: () => {
      if (widgetIdRef.current) {
        window.turnstile?.reset(widgetIdRef.current);
      }
    },
  }), []);

  // 若脚本已被其它实例加载（window.turnstile 已就绪），直接标记 ready。
  // 生产首次加载时 window.turnstile 初始不存在，由 <Script onLoad> 置 true。
  useEffect(() => {
    if (sitekey && window.turnstile && !scriptReady) {
      setScriptReady(true);
    }
  }, [sitekey, scriptReady]);

  useEffect(() => {
    if (!sitekey || !scriptReady) return;
    if (!window.turnstile || !containerRef.current) return;

    // StrictMode 双调用防护：已有 widget 先清理再渲染，避免重复 widget 泄漏。
    if (widgetIdRef.current) {
      window.turnstile.remove(widgetIdRef.current);
      widgetIdRef.current = null;
    }
    widgetIdRef.current = window.turnstile.render(containerRef.current, {
      sitekey,
      action,
      theme,
      callback: onToken,
      "error-callback": onError,
      "expired-callback": onExpire,
    });

    return () => {
      if (widgetIdRef.current) {
        window.turnstile?.remove(widgetIdRef.current);
        widgetIdRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sitekey, action, theme, scriptReady]);

  if (!sitekey) return null;

  return (
    <>
      <Script src={TURNSTILE_SCRIPT} strategy="afterInteractive" onLoad={() => setScriptReady(true)} />
      <div ref={containerRef} className={className} />
    </>
  );
});
