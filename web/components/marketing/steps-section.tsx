// 三步流程段：模板 → AGENT → 微信。
// 每个 step-cell 包含步骤号、标题、说明、伪终端样式的可视化块。

type Step = {
  num: string;
  title: string;
  desc: string;
  visual: { k: string; text: string; accent?: boolean }[];
};

const STEPS: Step[] = [
  {
    num: "STEP 01",
    title: "选模板，起名字",
    desc: "从公开模板库里挑一个人格——私人助理、学习陪伴、记账伙伴。给它取个名字，提交。",
    visual: [
      { k: "tmpl", text: "= assistant-zh@v3" },
      { k: "name", text: '= "我的私人助理"' },
      { k: "✓", text: "已锁定模板版本", accent: true },
    ],
  },
  {
    num: "STEP 02",
    title: "等待 Agent 进入 Running",
    desc: "平台异步供应：分配独立数据目录、复制 SOUL.md / USER.md / skills、启动专属运行环境。可以离开，回来再看。",
    visual: [
      { k: "› provision", text: "data dir · ok" },
      { k: "› inject  ", text: "persona · ok" },
      { k: "› status", text: "running", accent: true },
    ],
  },
  {
    num: "STEP 03",
    title: "扫码，绑定微信",
    desc: "启动微信绑定 → 实时拿到二维码 → 微信里扫一扫 → 凭据加密落盘。从此发消息，它就在微信里回。",
    visual: [
      { k: "› qr", text: "generated · 30s" },
      { k: "› scan", text: "by user · ok" },
      { k: "› wechat", text: "bound", accent: true },
    ],
  },
];

export default function StepsSection() {
  return (
    <section className="section steps-section" data-od-id="flow" id="flow">
      <div className="container">
        <p className="eyebrow">三步流程 · 模板 → AGENT → 微信</p>
        <h2>从一份模板，到活在微信里的 AI Agent。</h2>
        <p className="lead" style={{ marginTop: 18 }}>
          平台在后台依次完成数据目录分配、配置生成、专属运行环境启动。整个过程通常在数十秒内完成。
        </p>

        <div className="steps-grid">
          {STEPS.map((step) => (
            <div className="step-cell" key={step.num}>
              <div className="step-num">{step.num}</div>
              <h3>{step.title}</h3>
              <p>{step.desc}</p>
              <div className="step-visual">
                {step.visual.map((line, idx) =>
                  line.accent ? (
                    <div key={idx}>
                      <span className="acc">
                        {line.k} {line.text}
                      </span>
                    </div>
                  ) : (
                    <div key={idx}>
                      <span className="k">{line.k}</span> {line.text}
                    </div>
                  ),
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
