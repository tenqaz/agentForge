"use client";

import { useState } from "react";

// 微信扫码可视化演示段。
// 客户端组件：通过 stage state 驱动左侧步骤高亮与右侧手机内画面。
// 4 个阶段：qr（生成二维码）→ scanning（已扫描等待确认）→ confirmed（已确认）→ chat（开始接收消息）。
// 原 marketing.html 用 vanilla JS 操作 DOM，这里改写为 React 受控状态。

type Stage = "qr" | "scanning" | "confirmed" | "chat";

type StageItem = {
  stage: Stage;
  step: number;
  title: string;
  desc: string;
};

const STAGES: StageItem[] = [
  {
    stage: "qr",
    step: 1,
    title: "生成二维码",
    desc: "Agent 进入 Running 后，启动微信绑定，平台实时返回登录二维码与有效期。",
  },
  {
    stage: "scanning",
    step: 2,
    title: "用微信扫一扫",
    desc: "状态轮询从「等待扫码」切换到「等待确认」，二维码上出现已扫描覆盖。",
  },
  {
    stage: "confirmed",
    step: 3,
    title: "微信内确认绑定",
    desc: "登录凭据被加密保存到 Agent 的独立数据目录，凭据从不出现在前端或日志。",
  },
  {
    stage: "chat",
    step: 4,
    title: "开始接收微信消息",
    desc: "Agent 默认只回复扫码本人的私信。从此你给它发消息，它就在你熟悉的会话里回。",
  },
];

const STAGE_ORDER: Stage[] = ["qr", "scanning", "confirmed", "chat"];

// QR 码数据点坐标（来源于原 marketing.html，伪随机但确定的图案）。
// 三个定位点（角标）由 JSX 直接渲染；这里仅是中间的数据 module 坐标。
const QR_DATA_POINTS: Array<[number, number]> = [
  [8, 0], [10, 0], [12, 0],
  [9, 1], [11, 1],
  [8, 2], [10, 2], [13, 2],
  [9, 3], [12, 3],
  [8, 4], [11, 4], [13, 4],
  [10, 5], [12, 5],
  [0, 8], [2, 8], [4, 8], [7, 8], [9, 8], [11, 8], [14, 8], [16, 8], [18, 8], [20, 8],
  [1, 9], [3, 9], [6, 9], [8, 9], [10, 9], [13, 9], [15, 9], [17, 9], [19, 9],
  [0, 10], [2, 10], [5, 10], [7, 10], [11, 10], [13, 10], [16, 10], [18, 10],
  [1, 11], [4, 11], [6, 11], [9, 11], [12, 11], [14, 11], [17, 11], [19, 11],
  [0, 12], [3, 12], [5, 12], [8, 12], [10, 12], [13, 12], [15, 12], [18, 12], [20, 12],
  [2, 13], [4, 13], [7, 13], [9, 13], [11, 13], [13, 13], [16, 13], [19, 13],
  [8, 14], [10, 14], [13, 14], [15, 14], [17, 14], [20, 14],
  [9, 15], [12, 15], [14, 15], [18, 15],
  [8, 16], [11, 16], [13, 16], [16, 16], [19, 16],
  [9, 17], [14, 17], [17, 17], [20, 17],
  [10, 18], [12, 18], [15, 18], [18, 18],
  [9, 19], [11, 19], [13, 19], [16, 19], [19, 19],
  [10, 20], [14, 20], [17, 20], [20, 20],
];

const CHAT_MESSAGES: Array<{ kind: "in" | "out"; text: string }> = [
  { kind: "in", text: "你好！我现在已经活在你的微信里了 👋" },
  { kind: "out", text: "帮我把今天 14:30 的会议提醒" },
  { kind: "in", text: "好的，提醒已记下。要顺便把上次会议要点整理一下发你吗？" },
  { kind: "out", text: "先发要点。" },
];

function statusTextFor(stage: Stage): string {
  switch (stage) {
    case "qr":
      return "等待扫码 · 30s";
    case "scanning":
      return "已扫描 · 等待确认";
    case "confirmed":
      return "已确认 · 凭据加密落盘";
    default:
      return "";
  }
}

export default function WechatDemo() {
  const [stage, setStage] = useState<Stage>("qr");

  const stageIndex = STAGE_ORDER.indexOf(stage);
  const isChat = stage === "chat";
  const isScanned = stage === "scanning" || stage === "confirmed";
  const isConfirmed = stage === "confirmed";

  return (
    <section
      className="section wechat-section"
      data-od-id="wechat"
      id="wechat"
    >
      <div className="container wechat-grid">
        <div className="wechat-copy">
          <p className="eyebrow">微信扫码 · 可视化演示</p>
          <h2>四步绑定，从二维码到能聊。</h2>
          <p className="lead">
            点击右侧任意一步，看 Agent 与微信连接的完整过程。整段流程在平台内完成；凭据加密落盘后，运行时重建也不会要求你重新扫码。
          </p>

          <ul className="wechat-stage-list">
            {STAGES.map((item, idx) => {
              const classes: string[] = [];
              if (item.stage === stage) classes.push("is-active");
              else if (idx < stageIndex) classes.push("is-done");
              return (
                <li
                  key={item.stage}
                  className={classes.join(" ")}
                  data-stage={item.stage}
                  onClick={() => setStage(item.stage)}
                >
                  <span className="stage-step">{item.step}</span>
                  <div className="stage-text">
                    <strong>{item.title}</strong>
                    <span>{item.desc}</span>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>

        <div className="phone-wrap">
          <div className="phone">
            <div className="phone-notch"></div>
            <div className="phone-screen">
              <div className="phone-status">
                <span>9:41</span>
                <span className="signal">
                  <i></i>
                  <i></i>
                  <i></i>
                  <i></i>
                </span>
              </div>
              <div className="wx-app">
                <div className="wx-header">
                  <span className="wx-back">‹</span>
                  <span className="wx-title">
                    {isChat ? "我的私人助理" : "AgentForge · 绑定"}
                  </span>
                  <span className="wx-more">···</span>
                </div>

                {/* Stage: QR — 涵盖 qr / scanning / confirmed 三种状态 */}
                {!isChat && (
                  <div className="wx-stage is-active" data-stage="qr">
                    <div className="qr-card">
                      <div className="qr-title">用微信「扫一扫」绑定 Agent</div>
                      <div
                        className={`qr-box${isScanned ? " scanned" : ""}`}
                      >
                        <svg
                          className="qr-svg"
                          viewBox="0 0 21 21"
                          shapeRendering="crispEdges"
                          aria-hidden="true"
                        >
                          <rect width="21" height="21" fill="white" />
                          {/* 三个定位点 */}
                          <rect x="0" y="0" width="7" height="7" fill="black" />
                          <rect x="1" y="1" width="5" height="5" fill="white" />
                          <rect x="2" y="2" width="3" height="3" fill="black" />
                          <rect x="14" y="0" width="7" height="7" fill="black" />
                          <rect x="15" y="1" width="5" height="5" fill="white" />
                          <rect x="16" y="2" width="3" height="3" fill="black" />
                          <rect x="0" y="14" width="7" height="7" fill="black" />
                          <rect x="1" y="15" width="5" height="5" fill="white" />
                          <rect x="2" y="16" width="3" height="3" fill="black" />
                          {/* 数据点 */}
                          <g fill="black">
                            {QR_DATA_POINTS.map(([x, y]) => (
                              <rect
                                key={`${x}-${y}`}
                                x={x}
                                y={y}
                                width="1"
                                height="1"
                              />
                            ))}
                          </g>
                        </svg>
                        <div className="qr-overlay">
                          <div className="qr-overlay-tick">✓</div>
                        </div>
                      </div>
                      <div
                        className={`qr-status${isConfirmed ? " ok" : ""}`}
                      >
                        <span className="dot"></span>
                        <span>{statusTextFor(stage)}</span>
                      </div>
                    </div>
                  </div>
                )}

                {/* Stage: Chat —— 仅当 stage === chat 时挂载，触发消息入场动画 */}
                {isChat && (
                  <div
                    className="wx-stage chat is-active"
                    data-stage="chat-screen"
                  >
                    <div className="wx-chat">
                      {CHAT_MESSAGES.map((msg, idx) => (
                        <div key={idx} className={`wx-msg ${msg.kind}`}>
                          {msg.text}
                        </div>
                      ))}
                    </div>
                    <div className="wx-input">
                      <div className="ic"></div>
                      <div className="ib"></div>
                      <div className="ic"></div>
                      <div className="ic"></div>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
