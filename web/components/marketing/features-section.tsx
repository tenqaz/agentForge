import type { ReactNode } from "react";

// 功能矩阵段：9 个 feature 卡片，每个含 SVG 图标 + 标题 + 描述。
// SVG 内嵌为 JSX，避免外部资源依赖；图标视觉与原 marketing.html 一致。

type Feature = {
  title: string;
  desc: string;
  icon: ReactNode;
};

const ICON_PROPS = {
  viewBox: "0 0 24 24",
  fill: "none" as const,
  stroke: "currentColor",
  strokeWidth: 1.6,
};

const FEATURES: Feature[] = [
  {
    title: "模板化创建",
    desc: "从预置模板出发，几次点击就能创建一个独立运行的 Agent。版本被锁定，已创建的 Agent 不会因模板更新而被改坏。",
    icon: (
      <svg {...ICON_PROPS}>
        <rect x="3" y="4" width="18" height="14" rx="2" />
        <path d="M3 9h18" />
        <path d="M8 14h8" />
      </svg>
    ),
  },
  {
    title: "多 Agent 并行托管",
    desc: "每个 Agent 拥有独立的运行时和数据目录。工作助理、学习陪伴、生活记账互不干扰，端口、配置、日志都不会再互相打架。",
    icon: (
      <svg {...ICON_PROPS}>
        <rect x="4" y="4" width="6" height="6" rx="1" />
        <rect x="14" y="4" width="6" height="6" rx="1" />
        <rect x="4" y="14" width="6" height="6" rx="1" />
        <rect x="14" y="14" width="6" height="6" rx="1" />
      </svg>
    ),
  },
  {
    title: "微信扫码全流程",
    desc: "二维码生成、状态轮询、凭据加密落盘、网关启动、断线重连——全部在平台里完成。你只需要拿起手机扫一下。",
    icon: (
      <svg {...ICON_PROPS}>
        <path d="M5 12h6l2-3 2 6 2-3h2" />
        <circle cx="12" cy="12" r="9" />
      </svg>
    ),
  },
  {
    title: "数据持久化",
    desc: "Agent 的人格、记忆、对话历史、微信凭据全部保存在独立数据目录中。运行时容器销毁与重建，都不会让这些东西消失。",
    icon: (
      <svg {...ICON_PROPS}>
        <path d="M4 7l8-4 8 4v8l-8 4-8-4V7z" />
        <path d="M12 11v8" />
        <path d="M4 7l8 4 8-4" />
      </svg>
    ),
  },
  {
    title: "多用户隔离",
    desc: "每个用户只能访问自己的 Agent。任何人都无法读取、修改、操作他人的 Agent 数据或运行时——隔离是平台的硬约束。",
    icon: (
      <svg {...ICON_PROPS}>
        <rect x="5" y="11" width="14" height="9" rx="2" />
        <path d="M8 11V8a4 4 0 0 1 8 0v3" />
      </svg>
    ),
  },
  {
    title: "异步供应 · 错误可恢复",
    desc: "从分配数据目录到运行时启动是一条可观测的事件流。Agent 启动失败、扫码超时都能进入可恢复状态，平台提供清晰的重试入口。",
    icon: (
      <svg {...ICON_PROPS}>
        <path d="M12 3l9 4-9 4-9-4 9-4z" />
        <path d="M3 12l9 4 9-4" />
        <path d="M3 17l9 4 9-4" />
      </svg>
    ),
  },
  {
    title: "密钥仅在服务端",
    desc: "模型 API key、微信凭据、平台 token 只在服务端使用。前端响应、日志、错误信息里看不到任何敏感凭据。",
    icon: (
      <svg {...ICON_PROPS}>
        <circle cx="12" cy="12" r="9" />
        <path d="M9 12l2 2 4-5" />
      </svg>
    ),
  },
  {
    title: "运行历史与状态可见",
    desc: "查看 Agent 的运行状态、关键事件流、错误信息。它今天有没有掉线、为什么掉线，一眼就能看明白。",
    icon: (
      <svg {...ICON_PROPS}>
        <path d="M3 12h4l3-7 4 14 3-7h4" />
      </svg>
    ),
  },
  {
    title: "默认收敛的安全策略",
    desc: "仅扫码本人发起的私信被回复，群聊默认禁用。让 Agent 留在你的会话里，而不是被陌生人或群聊滥用。",
    icon: (
      <svg {...ICON_PROPS}>
        <path d="M4 7h16" />
        <path d="M4 12h10" />
        <path d="M4 17h16" />
      </svg>
    ),
  },
];

export default function FeaturesSection() {
  return (
    <section
      className="section features-section"
      data-od-id="features"
      id="features"
    >
      <div className="container stack" style={{ gap: "var(--gap-xl)" }}>
        <div style={{ maxWidth: "38ch" }}>
          <p className="eyebrow">为什么选 AgentForge</p>
          <h2>把 Agent 工程化里那些恶心的东西，全部收敛进平台。</h2>
        </div>

        <div className="grid-3">
          {FEATURES.map((feature) => (
            <div className="feature card-flat" key={feature.title}>
              <div className="feature-mark">{feature.icon}</div>
              <h3>{feature.title}</h3>
              <p>{feature.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
