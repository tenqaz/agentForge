import { readFile } from "node:fs/promises";
import { join } from "node:path";
import "./marketing.css";

// 营销着陆页：直接还原参考 marketing.html（自包含静态页）。
// 公开页，不强制重定向；已登录用户在页内 CTA（登录/注册）触达控制台。
// CSS（marketing.css）含 marketing 专属类，按路由加载，与全局组件类隔离。
async function getMarketingBody(): Promise<string> {
  const filePath = join(process.cwd(), "app", "marketing-body.html");
  return readFile(filePath, "utf8");
}

export default async function MarketingPage() {
  const html = await getMarketingBody();
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}
