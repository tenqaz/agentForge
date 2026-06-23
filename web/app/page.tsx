import { readFile } from "node:fs/promises";
import { join } from "node:path";
import "./marketing.css";
import MarketingTopNav from "@/components/marketing-top-nav";

// 营销着陆页：还原参考 marketing.html 的视觉。
// - 顶部导航（含「登录/免费创建 Agent」按钮）由 MarketingTopNav 客户端组件提供，
//   按当前会话切换为「进入控制台/退出」。
// - 余下章节仍由静态 HTML 渲染；hero/footer 的 CTA <button> 在此替换为带路由的 <a>，
//   保留原 class 与文案。
async function getMarketingBody(): Promise<string> {
  const filePath = join(process.cwd(), "app", "marketing-body.html");
  let html = await readFile(filePath, "utf8");

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
  return (
    <>
      <MarketingTopNav />
      <div dangerouslySetInnerHTML={{ __html: html }} />
    </>
  );
}
