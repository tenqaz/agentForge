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

const PUBLIC_PATHS = ["/login", "/register"];

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
          <div className="min-h-screen bg-[color:var(--color-bg)] text-[color:var(--color-fg)]">
            {/* Desktop sidebar — visual only at the chrome layer; pointer-events restored on the inner Sidebar so its interactive children remain clickable while the main content remains hit-test reachable. */}
            <aside className="pointer-events-none fixed inset-y-0 left-0 z-30 hidden w-[var(--sidebar-width)] border-r border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)] lg:block">
              <div className="pointer-events-auto h-full">
                <Sidebar
                  user={user}
                  loading={loading}
                  onSignOut={handleSignOut}
                  pathname={pathname}
                />
              </div>
            </aside>

            {/* Mobile top bar */}
            <header className="fixed inset-x-0 top-0 z-40 h-[var(--topbar-height)] border-b border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)] px-4 lg:hidden">
              <MobileTopBar
                onOpenDrawer={() => setDrawerOpen(true)}
                user={user}
              />
            </header>

            {/* Mobile drawer */}
            <MobileDrawer
              open={drawerOpen}
              onClose={() => setDrawerOpen(false)}
              user={user}
              loading={loading}
              onSignOut={handleSignOut}
              pathname={pathname}
            />

            {/* Content (isolate creates a new stacking context so the fixed sidebar never sits on top of interactive content) */}
            <main className="relative isolate pt-[var(--topbar-height)] lg:pt-0 lg:pl-[var(--sidebar-width)]">
              <div className="mx-auto w-full max-w-[var(--content-max)] px-4 py-6 sm:px-6 lg:px-10 lg:py-10">
                {children}
              </div>
            </main>
          </div>
        ) : (
          <div className="min-h-screen bg-[color:var(--color-bg)] text-[color:var(--color-fg)]">
            <div className="mx-auto w-full max-w-[var(--content-max)] px-4 py-10 sm:px-6">
              {children}
            </div>
          </div>
        )}
      </SessionContext.Provider>
    </ApiClientContext.Provider>
  );
}
