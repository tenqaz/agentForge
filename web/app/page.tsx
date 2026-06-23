import "./marketing.css";
import MarketingTopNav from "@/components/marketing-top-nav";
import HeroSection from "@/components/marketing/hero-section";
import StepsSection from "@/components/marketing/steps-section";
import FeaturesSection from "@/components/marketing/features-section";
import UsecaseSection from "@/components/marketing/usecase-section";
import WechatDemo from "@/components/marketing/wechat-demo";
import RoadmapSection from "@/components/marketing/roadmap-section";
import CtaSection from "@/components/marketing/cta-section";
import MarketingFooter from "@/components/marketing/marketing-footer";

// 营销着陆页：整页由 React 组件树渲染，样式仍统一来自 marketing.css。
// 顶部导航 MarketingTopNav 是客户端组件，根据登录态切换右上角按钮；
// WechatDemo 同样是客户端组件，承载阶段切换交互；其余段保持服务端渲染。
export default function MarketingPage() {
  return (
    <>
      <MarketingTopNav />
      <main id="content">
        <HeroSection />
        <StepsSection />
        <FeaturesSection />
        <UsecaseSection />
        <WechatDemo />
        <RoadmapSection />
        <CtaSection />
      </main>
      <MarketingFooter />
    </>
  );
}
