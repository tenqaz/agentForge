import { readFile } from "node:fs/promises";
import { join } from "node:path";
import "./marketing.css";

// 营销着陆页：还原参考 marketing.html 的视觉（自包含静态页）。
// 公开页，不强制重定向。参考原型里的 CTA 是无行为的 <button>，这里替换成
// 真实路由 <a>，使"登录/免费创建 Agent/浏览公开模板"可导航到对应功能页。
async function getMarketingBody(): Promise<string> {
  const filePath = join(process.cwd(), "app", "marketing-body.html");
  let html = await readFile(filePath, "utf8");

  // 把无行为的 CTA <button> 替换为带路由的 <a>，保留原 class 与文案。
  // 「登录」→ /login；「免费创建 Agent」→ /register；「浏览公开模板」→ /templates
  html = html.replace(
    /<button([^>]*class="([^"]*btn[^"]*)")>\s*登录\s*<\/button>/g,
    '<a$1 href="/login">登录</a>',
  );
  html = html.replace(
    /<button([^>]*class="([^"]*btn[^"]*)")>\s*免费创建 Agent\s*<\/button>/g,
    '<a$1 href="/register">免费创建 Agent</a>',
  );
  html = html.replace(
    /<button([^>]*class="([^"]*btn[^"]*)")>\s*浏览公开模板\s*<\/button>/g,
    '<a$1 href="/templates">浏览公开模板</a>',
  );

  return html;
}

export default async function MarketingPage() {
  const html = await getMarketingBody();
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}
