"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { ArrowRight, Bot, LayoutTemplate, ShieldCheck, Workflow } from "lucide-react";

import { useSessionState } from "@/components/app-shell";
import Button from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export default function Home() {
  const router = useRouter();
  const { loading, user } = useSessionState();

  useEffect(() => {
    if (loading) {
      return;
    }
    if (user?.role === "admin") {
      router.replace("/admin/templates");
      return;
    }
    if (user) {
      router.replace("/templates");
    }
  }, [loading, router, user]);

  return (
    <section className="grid gap-10 py-10 lg:grid-cols-[1.25fr_0.85fr] lg:py-16">
      <div className="flex flex-col justify-center">
        <span className="inline-flex w-fit items-center gap-2 rounded-full border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)] px-3 py-1 text-xs font-medium text-[color:var(--color-fg-muted)]">
          <span
            className="size-1.5 rounded-full bg-[color:var(--color-success)]"
            aria-hidden="true"
          />
          运营控制台
        </span>
        <h1 className="mt-6 max-w-2xl text-4xl font-semibold leading-[1.1] tracking-tight text-[color:var(--color-fg)] sm:text-5xl lg:text-[56px]">
          启动 Agent，发布 Template，完成微信配对，全在一处。
        </h1>
        <p className="mt-6 max-w-xl text-base leading-7 text-[color:var(--color-fg-muted)]">
          AgentForge 把运行时状态、模板生命周期、渠道激活整合到一个面板。管理员塑造模板，运营者启动 Agent 并将其推向已连接的渠道。
        </p>
        <div className="mt-9 flex flex-wrap gap-3">
          <Link href="/login">
            <Button variant="primary" rightIcon={<ArrowRight size={16} strokeWidth={1.75} />}>
              登录
            </Button>
          </Link>
          <Link href="/templates">
            <Button variant="secondary">浏览 Templates</Button>
          </Link>
        </div>
      </div>

      <div className="grid gap-4 self-center">
        <Card>
          <div className="flex items-start gap-3">
            <span
              className="grid size-9 shrink-0 place-items-center rounded-[var(--radius-md)] bg-[color:var(--color-accent-soft)] text-[color:var(--color-accent)]"
              aria-hidden="true"
            >
              <LayoutTemplate size={18} strokeWidth={1.75} />
            </span>
            <div>
              <h3 className="text-sm font-semibold text-[color:var(--color-fg)]">Template 全生命周期</h3>
              <p className="mt-1.5 text-sm leading-6 text-[color:var(--color-fg-muted)]">
                管理员可对 SOUL.md、USER.md 与整套技能进行增删改查，发布即对运营者可见。
              </p>
            </div>
          </div>
        </Card>
        <Card>
          <div className="flex items-start gap-3">
            <span
              className="grid size-9 shrink-0 place-items-center rounded-[var(--radius-md)] bg-[color:var(--color-info-soft)] text-[color:var(--color-info)]"
              aria-hidden="true"
            >
              <Bot size={18} strokeWidth={1.75} />
            </span>
            <div>
              <h3 className="text-sm font-semibold text-[color:var(--color-fg)]">Agent 运行时进度</h3>
              <p className="mt-1.5 text-sm leading-6 text-[color:var(--color-fg-muted)]">
                从创建到运行中的每个阶段都会自动刷新，错误信息直接可见，方便快速定位。
              </p>
            </div>
          </div>
        </Card>
        <Card>
          <div className="flex items-start gap-3">
            <span
              className="grid size-9 shrink-0 place-items-center rounded-[var(--radius-md)] bg-[color:var(--color-success-soft)] text-[color:var(--color-success)]"
              aria-hidden="true"
            >
              <Workflow size={18} strokeWidth={1.75} />
            </span>
            <div>
              <h3 className="text-sm font-semibold text-[color:var(--color-fg)]">微信配对开箱即用</h3>
              <p className="mt-1.5 text-sm leading-6 text-[color:var(--color-fg-muted)]">
                生成二维码，等待扫码，连接状态全程可见，不暴露任何令牌。
              </p>
            </div>
          </div>
        </Card>
        <div className="flex items-center gap-3 rounded-[var(--radius-xl)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)]/50 px-5 py-4 text-xs leading-5 text-[color:var(--color-fg-muted)]">
          <ShieldCheck
            size={16}
            strokeWidth={1.75}
            className="shrink-0 text-[color:var(--color-fg-subtle)]"
            aria-hidden="true"
          />
          密钥保留在服务端，运行时未就绪前微信控制保持禁用。
        </div>
      </div>
    </section>
  );
}
