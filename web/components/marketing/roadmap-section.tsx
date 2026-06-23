// MVP 里程碑与规划段。分为已交付和有计划两列。
// 每个条目含 check 图标、标签文字、标签徽章。

type RoadmapItem = {
  done: boolean;
  label: string;
};

const DONE: RoadmapItem[] = [
  { done: true, label: "邮箱注册 / 登录 / 会话管理" },
  { done: true, label: "默认管理员账号" },
  { done: true, label: "管理员模板 CRUD（SOUL.md / USER.md / skills）" },
  { done: true, label: "模板发布 · 归档 · 版本锁定" },
  { done: true, label: "基于模板创建 Agent" },
  { done: true, label: "Agent 异步供应：目录 → 配置 → 运行时" },
  { done: true, label: "运行状态查询与历史事件追踪" },
  { done: true, label: "Agent 删除（含运行时与数据回收）" },
  { done: true, label: "微信扫码绑定全流程 · 状态展示 · 断开重连" },
  { done: true, label: "Next.js 前端控制台" },
];

const NEXT: RoadmapItem[] = [
  { done: false, label: "普通用户上传 / 编辑自定义 skills" },
  { done: false, label: "用户编辑 SOUL.md / USER.md" },
  { done: false, label: "模板变量（不改模板做轻定制）" },
  { done: false, label: "QQ / Telegram / 企业微信渠道" },
  { done: false, label: "模板市场 · 评分 · 分享" },
  { done: false, label: "团队 / 组织 / 多人协作" },
  { done: false, label: "Agent 在用户之间分享" },
  { done: false, label: "Agent 运行成本与资源用量统计" },
];

export default function RoadmapSection() {
  return (
    <section
      className="section roadmap-section"
      data-od-id="roadmap"
      id="roadmap"
    >
      <div className="container">
        <p className="eyebrow">里程碑 · 2026-06-19 快照</p>
        <h2>已交付的主链路，与还在路上的能力。</h2>
        <p className="lead" style={{ marginTop: 18 }}>
          MVP 聚焦于「模板 → Agent → 微信」一条主链路稳定可用。其它能力会在主链路稳定后逐步开放。
        </p>

        <div className="roadmap-grid">
          <div className="roadmap-col">
            <h3>MVP 已交付（2026-06-19）</h3>
            <ul className="roadmap-list">
              {DONE.map((item) => (
                <li key={item.label}>
                  <span className="check-icon done">✓</span>
                  <span className="label">{item.label}</span>
                  <span className="tag tag-done">DONE</span>
                </li>
              ))}
            </ul>
          </div>

          <div className="roadmap-col">
            <h3>主链路稳定后逐步开放</h3>
            <ul className="roadmap-list">
              {NEXT.map((item) => (
                <li key={item.label}>
                  <span className="check-icon next">·</span>
                  <span className="label label-soon">{item.label}</span>
                  <span className="tag tag-soon">NEXT</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}