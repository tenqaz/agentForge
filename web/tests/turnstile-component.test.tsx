// @vitest-environment jsdom

import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render } from "@testing-library/react";
import { createRef } from "react";

import { Turnstile } from "@/components/turnstile";

type TurnstileRender = (container: string | HTMLElement, options: Record<string, unknown>) => string;

declare global {
  interface Window {
    turnstile?: {
      render: TurnstileRender;
      reset: (id?: string) => void;
      remove: (id: string) => void;
    };
  }
}

function installTurnstileMock() {
  const renderMock = vi.fn<TurnstileRender>(() => "widget-1");
  window.turnstile = {
    render: renderMock,
    reset: vi.fn(),
    remove: vi.fn(),
  };
  return { renderMock };
}

describe("Turnstile component", () => {
  beforeEach(() => {
    delete (window as { turnstile?: unknown }).turnstile;
  });
  afterEach(() => {
    delete (window as { turnstile?: unknown }).turnstile;
    vi.restoreAllMocks();
  });

  it("renders null when sitekey is empty", () => {
    const { renderMock } = installTurnstileMock();
    const { container } = render(<Turnstile sitekey="" action="login" onToken={() => {}} />);
    expect(container.firstChild).toBeNull();
    expect(renderMock).not.toHaveBeenCalled();
  });

  it("renders container and calls turnstile.render with sitekey and action", () => {
    const { renderMock } = installTurnstileMock();
    render(<Turnstile sitekey="site" action="login" onToken={() => {}} />);
    expect(renderMock).toHaveBeenCalledTimes(1);
    const options = renderMock.mock.calls[0][1] as Record<string, unknown>;
    expect(options.sitekey).toBe("site");
    expect(options.action).toBe("login");
  });

  it("invokes onToken via the callback option", () => {
    const { renderMock } = installTurnstileMock();
    const onToken = vi.fn();
    render(<Turnstile sitekey="site" action="login" onToken={onToken} />);
    const options = renderMock.mock.calls[0][1] as { callback: (token: string) => void };
    options.callback("the-token");
    expect(onToken).toHaveBeenCalledWith("the-token");
  });

  it("reset() calls turnstile.reset with widgetId", () => {
    installTurnstileMock();
    const ref = createRef<{ reset: () => void }>();
    render(<Turnstile ref={ref} sitekey="site" action="login" onToken={() => {}} />);
    ref.current?.reset();
    expect(window.turnstile?.reset).toHaveBeenCalledWith("widget-1");
  });
});
