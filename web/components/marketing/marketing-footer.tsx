// 营销页页脚：品牌简介 + 四列链接 + 底部版权。

export default function MarketingFooter() {
  return (
    <footer className="pagefoot" data-od-id="footer">
      <div className="container">
        <div className="foot-col foot-brand">
          <span
            className="logo"
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              fontFamily: "var(--font-display)",
              fontWeight: 600,
              fontSize: 17,
            }}
          >
            <span
              className="logo-mark"
              style={{
                width: 22,
                height: 22,
                borderRadius: 6,
                background: "var(--fg)",
                display: "inline-grid",
                placeItems: "center",
                color: "var(--surface)",
                fontFamily: "var(--font-mono)",
                fontSize: 12,
              }}
            >
              A
            </span>
            AgentForge
          </span>
          <p>
            面向个人开发者的 AI Agent 托管平台。从模板出发，几次点击拥有一个活在你微信里的 Agent。
          </p>
        </div>
        <div className="foot-col">
          <h4>产品</h4>
          <ul>
            <li>
              <a href="#features">功能</a>
            </li>
            <li>
              <a href="#flow">三步流程</a>
            </li>
            <li>
              <a href="#wechat">微信接入</a>
            </li>
            <li>
              <a href="#roadmap">里程碑</a>
            </li>
          </ul>
        </div>
        <div className="foot-col">
          <h4>开发者</h4>
          <ul>
            <li>
              <a href="#">公开模板</a>
            </li>
            <li>
              <a href="#">SOUL.md 规范</a>
            </li>
            <li>
              <a href="#">技术设计文档</a>
            </li>
            <li>
              <a href="#">事件流参考</a>
            </li>
          </ul>
        </div>
        <div className="foot-col">
          <h4>关于</h4>
          <ul>
            <li>
              <a href="#">联系我们</a>
            </li>
            <li>
              <a href="#">隐私 · 数据隔离</a>
            </li>
            <li>
              <a href="#">安全策略</a>
            </li>
            <li>
              <a href="#">更新日志</a>
            </li>
          </ul>
        </div>
      </div>
      <div className="pagefoot-bottom">
        <span>© AgentForge · 2026 · MVP 版本快照 · 2026-06-19</span>
        <span className="meta">
          made for individual developers, served as a platform
        </span>
      </div>
    </footer>
  );
}