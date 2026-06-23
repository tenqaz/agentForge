import type { ReactNode } from "react";

// 典型场景段：三类个人开发者使用方式。
// flow 字段使用 ReactNode，方便用 <b> 强调关键词。

type Usecase = {
  tag: string;
  title: string;
  desc: string;
  flow: ReactNode;
};

const USECASES: Usecase[] = [
  {
    tag: "场景 01 · 验证想法",
    title: "给一个 Agent 创意一周不到的验证窗口。",
    desc: "独立开发者有一个 Agent 创意，但不想花一周时间研究框架、写运行时、接消息平台。用 AgentForge：选模板 → 命名 → 扫码。今晚就能在微信里跟它对话验证。",
    flow: (
      <>
        想法 → <b>模板</b> → <b>Agent</b> → <b>微信</b> · 一晚
        <br />
        不写代码 · 不配 YAML · 不开 VPS
      </>
    ),
  },
  {
    tag: "场景 02 · 长期托管",
    title: "会写 Agent 的极客，懒得维护服务器。",
    desc: "已经会写 Agent，但不想再自己维护服务器、容器、二维码登录态、断线重连。把这些交给平台：异步供应、独立运行时、断线自动恢复，加密凭据持久化。",
    flow: (
      <>
        <b>专注 Agent 行为</b> 而不是基础设施
        <br />
        升级运行时 → 数据目录不变 · 无需重新扫码
      </>
    ),
  },
  {
    tag: "场景 03 · 自然交互",
    title: "让 AI 进入你已经在用的微信会话。",
    desc: "不再开 App、网页、ChatGPT 标签页。Agent 就活在你的微信会话里——给它发消息像给朋友发消息，回复进入聊天列表，跟其他对话排在一起。它默认只回复你本人，群聊默认禁用。",
    flow: (
      <>
        微信私信 → <b>本人专属</b> · 群聊默认禁用
        <br />
        凭据加密 · 不出现在前端 / 日志
      </>
    ),
  },
];

export default function UsecaseSection() {
  return (
    <section
      className="section usecase-section"
      data-od-id="cases"
      id="cases"
    >
      <div className="container">
        <p className="eyebrow">典型场景 · 三类个人开发者</p>
        <h2>三种把 Agent 装进生活的方式。</h2>

        <div className="usecase-grid">
          {USECASES.map((uc) => (
            <article className="usecase-card" key={uc.tag}>
              <span className="uc-tag">{uc.tag}</span>
              <h3>{uc.title}</h3>
              <p>{uc.desc}</p>
              <div className="uc-flow">{uc.flow}</div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
