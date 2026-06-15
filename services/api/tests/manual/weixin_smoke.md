# Manual Weixin Smoke Test

1. 启动 Docker。
2. 在 `services/api` 执行 `go run ./cmd/agentforge-api`。
3. 在 `web` 执行 `npm run dev`。
4. 管理员创建并发布模板。
5. 普通用户创建 Agent。
6. 等待 Agent 进入 `running`。
7. 创建微信 pairing session。
8. 用后续要给 Agent 发消息的微信账号扫码。
9. 在微信内确认。
10. 给 Agent 发私信。
11. 确认微信中收到 Agent 回复。
