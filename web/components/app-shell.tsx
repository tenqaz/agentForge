"use client";

import { usePathname, useRouter } from "next/navigation";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";

import {
  createApiClient,
  getSession,
  type ApiClient,
  type User,
} from "@/lib/api";
import MobileDrawer from "@/components/ui/mobile-drawer";
import MobileTopBar from "@/components/ui/mobile-top-bar";
import Sidebar from "@/components/ui/sidebar";

type AppShellProps = {
  children: React.ReactNode;
  apiBaseUrl?: string;
};

type SessionState = {
  loading: boolean;
  user: User | null;
  refreshSession: () => Promise<User | null>;
  clearSession: () => void;
};

const ApiClientContext = createContext<ApiClient | null>(null);
const SessionContext = createContext<SessionState | null>(null);

export function useApiClient() {
  const client = useContext(ApiClientContext);
  if (!client) {
    throw new Error("useApiClient must be used within AppShell");
  }
  return client;
}

export function useSessionState() {
  const session = useContext(SessionContext);
  if (!session) {
    throw new Error("useSessionState must be used within AppShell");
  }
  return session;
}

// 公开路径：营销页、登录、注册、管理员登录 —— 不套 chrome（sidebar/topbar），
// 由页面自身提供布局（营销页全屏 / auth 页 .auth split）。
const PUBLIC_PATHS = ["/", "/login", "/register", "/admin/login"];

export default function AppShell({ children, apiBaseUrl }: AppShellProps) {
  const pathname = usePathname();
  const router = useRouter();
  const [apiClient] = useState(() =>
    createApiClient({
      baseUrl: apiBaseUrl ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? "",
      defaultHeaders: {
        accept: "application/json",
      },
    }),
  );
  const [loading, setLoading] = useState(true);
  const [user, setUser] = useState<User | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const refreshSession = useCallback(async () => {
    setLoading(true);
    const response = await getSession(apiClient);
    if (response.ok) {
      setUser(response.data.user);
      setLoading(false);
      return response.data.user;
    }
    if (response.status === 401) {
      setUser(null);
      setLoading(false);
      return null;
    }
    setLoading(false);
    throw new Error(response.error.code ?? response.error.message);
  }, [apiClient]);

  useEffect(() => {
    void (async () => {
      await refreshSession();
    })();
  }, [refreshSession]);

  // Close drawer on route change. setState within effect is intentional here:
  // the drawer's open state is derived from external navigation events.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDrawerOpen(false);
  }, [pathname]);

  async function handleSignOut() {
    await apiClient.delete("/api/session");
    setUser(null);
    router.push("/login");
    router.refresh();
  }

  const isPublicPath = PUBLIC_PATHS.some(
    (p) => pathname === p || pathname.startsWith(`${p}/`),
  );
  // 仅登录用户 + 非公开路径才套 chrome；公开页（营销/auth）自行渲染布局。
  const showShell = !!user && !isPublicPath;

  return (
    <ApiClientContext.Provider value={apiClient}>
      <SessionContext.Provider
        value={{
          loading,
          user,
          refreshSession,
          clearSession: () => setUser(null),
        }}
      >
        {showShell ? (
          <div className="app">
            {/* Desktop sidebar（.app grid 240px 列；.sidebar 自带 sticky/hidden@mobile） */}
            <Sidebar
              user={user}
              loading={loading}
              onSignOut={handleSignOut}
              pathname={pathname}
            />

            <div className="main">
              {/* Topbar：移动端显示菜单按钮触发 drawer，桌面端由各页 topbar-trail 填充 */}
              <header className="topbar">
                <MobileTopBar
                  onOpenDrawer={() => setDrawerOpen(true)}
                  user={user}
                />
              </header>

              <MobileDrawer
                open={drawerOpen}
                onClose={() => setDrawerOpen(false)}
                user={user}
                loading={loading}
                onSignOut={handleSignOut}
                pathname={pathname}
              />

              <main className="page">{children}</main>
            </div>
          </div>
        ) : (
          // 公开页：不套 chrome，背景由 body 提供，布局由页面自行处理。
          children
        )}
      </SessionContext.Provider>
    </ApiClientContext.Provider>
  );
}
