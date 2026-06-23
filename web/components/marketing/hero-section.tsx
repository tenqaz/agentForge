import Link from "next/link";

// 营销首页 Hero 段：左侧文案 + 主/次 CTA + 关键数据；右侧 Agent fleet 插画。
// CTA 直接渲染为带路由的 <Link>，沿用 marketing.css 中 .btn 系列样式。

type FleetRow = {
  icon: string;
  name: string;
  tmpl: string;
  status: "RUNNING" | "STARTING" | "—";
  meta: string;
  variant: "running" | "starting" | "idle";
  accent?: boolean;
  muted?: boolean;
};

const FLEET_ROWS: FleetRow[] = [
  {
    icon: "私",
    name: "私人助理",
    tmpl: "tmpl: assistant-zh · v3",
    status: "RUNNING",
    meta: "微信已绑",
    variant: "running",
    accent: true,
  },
  {
    icon: "学",
    name: "学习陪伴",
    tmpl: "tmpl: study-coach · v1",
    status: "RUNNING",
    meta: "微信已绑",
    variant: "running",
  },
  {
    icon: "记",
    name: "生活记账",
    tmpl: "tmpl: life-ledger · v2",
    status: "RUNNING",
    meta: "微信已绑",
    variant: "running",
  },
  {
    icon: "读",
    name: "读书笔记",
    tmpl: "tmpl: reading-notes · v1",
    status: "STARTING",
    meta: "准备中",
    variant: "starting",
  },
  {
    icon: "+",
    name: "从模板新建…",
    tmpl: "选择一个公开模板",
    status: "—",
    meta: "—",
    variant: "idle",
    muted: true,
  },
];

export default function HeroSection() {
  return (
    <section className="section hero" data-od-id="hero">
      <div className="container hero-split">
        <div>
          <div className="hero-eyebrow-row">
            <span className="status-dot"></span>
            <span className="hero-status">
              <b>MVP 已上线</b> · 主链路稳定可用 · 2026-06
            </span>
          </div>
          <h1>
            把「做出一个能用的 AI Agent」<br />
            从工程项目，<span className="accent-word">压缩成一次配置</span>。
          </h1>
          <p className="lead" style={{ marginTop: 22 }}>
            从模板出发、几次点击拥有一个独立运行的 AI Agent；扫一次微信码，它就活在你的对话列表里——能收消息、能回复、能记住你。不写一行代码，不碰一行 YAML。
          </p>
          <div className="hero-cta" style={{ marginTop: 32 }}>
            <Link className="btn btn-primary" href="/register">
              免费创建 Agent
            </Link>
            <Link className="btn btn-secondary btn-arrow" href="/templates">
              浏览公开模板
            </Link>
          </div>
          <div className="hero-meta-row">
            <div className="hero-meta-item">
              <span className="num">3 步</span>
              <span>从模板到能在微信里聊</span>
            </div>
            <div className="hero-meta-item">
              <span className="num">N+</span>
              <span>并行托管多个独立 Agent</span>
            </div>
            <div className="hero-meta-item">
              <span className="num">0 行</span>
              <span>需要你写的部署代码</span>
            </div>
          </div>
        </div>

        <div className="agent-fleet" aria-label="并行托管多个 Agent 的示意">
          <div className="agent-fleet-grid">
            <div className="fleet-head">
              <div className="fleet-title">
                <span className="fleet-dot"></span>
                我的 Agents
              </div>
              <div className="fleet-controls">
                <span></span>
                <span></span>
                <span></span>
              </div>
            </div>
            <div className="fleet-list">
              {FLEET_ROWS.map((row, idx) => (
                <div
                  key={idx}
                  className={`fleet-row ${row.variant}${row.accent ? " is-accent" : ""}`}
                >
                  <div className="fleet-icon">{row.icon}</div>
                  <div>
                    <div
                      className="fleet-name"
                      style={
                        row.muted
                          ? { color: "var(--muted)", fontWeight: 500 }
                          : undefined
                      }
                    >
                      {row.name}
                    </div>
                    <div className="fleet-tmpl">{row.tmpl}</div>
                  </div>
                  <div className="fleet-status">
                    <span className="status-dot-sm"></span>
                    {row.status}
                  </div>
                  <div className="meta">{row.meta}</div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
