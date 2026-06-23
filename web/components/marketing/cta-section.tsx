import Link from "next/link";

// 页面底部 CTA 条：标题 + 说明 + 行动点 + 小字免责。

export default function CtaSection() {
  return (
    <section className="cta-strip" data-od-id="cta">
      <div className="container" style={{ maxWidth: 720 }}>
        <p className="eyebrow">现在就开始</p>
        <h2>
          你的下一个 AI Agent，
          <br />
          不该再是一个工程项目。
        </h2>
        <p className="lead">
          注册账号、选模板、扫码——今晚就让一个 Agent 活在你的微信里。
        </p>
        <div className="hero-cta" style={{ justifyContent: "center" }}>
          <Link className="btn btn-primary" href="/register">
            免费创建 Agent
          </Link>
          <Link className="btn btn-secondary btn-arrow" href="/templates">
            浏览公开模板
          </Link>
        </div>
        <p className="meta">无需信用卡 · 邮箱注册 · 多 Agent 默认隔离</p>
      </div>
    </section>
  );
}