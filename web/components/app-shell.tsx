"use client";

import Link from "next/link";
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

  async function handleSignOut() {
    await apiClient.delete("/api/session");
    setUser(null);
    router.push("/login");
    router.refresh();
  }

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
        <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,_rgba(186,107,34,0.18),_transparent_32%),linear-gradient(180deg,_#f6efe3_0%,_#f2ebdf_55%,_#efe6d8_100%)] text-stone-900">
          <div className="mx-auto flex min-h-screen w-full max-w-7xl flex-col px-4 pb-8 pt-4 sm:px-6 lg:px-10">
            <header className="rounded-[2rem] border border-stone-900/10 bg-white/70 px-5 py-4 shadow-[0_20px_80px_rgba(35,28,19,0.08)] backdrop-blur">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
                <div className="flex items-center gap-4">
                  <Link
                    className="inline-flex items-center gap-3 text-sm font-semibold uppercase tracking-[0.28em] text-stone-700"
                    href="/"
                  >
                    <span className="inline-flex h-11 w-11 items-center justify-center rounded-full bg-stone-900 text-sm text-stone-50">
                      AF
                    </span>
                    AgentForge Console
                  </Link>
                  <span className="hidden rounded-full border border-stone-900/10 bg-stone-50 px-3 py-1 text-xs font-medium uppercase tracking-[0.2em] text-stone-500 md:inline-flex">
                    MVP
                  </span>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <nav className="flex flex-wrap items-center gap-2 text-sm">
                    <NavLink href="/templates" active={pathname.startsWith("/templates")}>
                      Templates
                    </NavLink>
                    <NavLink href="/agents" active={pathname.startsWith("/agents")}>
                      Agents
                    </NavLink>
                    {user?.role === "admin" ? (
                      <NavLink
                        href="/admin/templates"
                        active={pathname.startsWith("/admin/templates")}
                      >
                        Admin
                      </NavLink>
                    ) : null}
                  </nav>
                  {loading ? (
                    <div className="rounded-full border border-stone-900/10 bg-stone-50 px-4 py-2 text-xs uppercase tracking-[0.18em] text-stone-500">
                      Loading session
                    </div>
                  ) : user ? (
                    <div className="flex items-center gap-3 rounded-full border border-stone-900/10 bg-stone-50 px-3 py-2">
                      <div className="text-right">
                        <p className="text-sm font-medium text-stone-900">{user.email}</p>
                        <p className="text-xs uppercase tracking-[0.2em] text-stone-500">
                          {user.role}
                        </p>
                      </div>
                      <button
                        className="rounded-full border border-stone-900/15 px-3 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-stone-700 transition hover:border-stone-900 hover:bg-stone-900 hover:text-stone-50"
                        onClick={() => void handleSignOut()}
                        type="button"
                      >
                        Sign out
                      </button>
                    </div>
                  ) : (
                    <NavLink href="/login" active={pathname.startsWith("/login")}>
                      Sign in
                    </NavLink>
                  )}
                </div>
              </div>
            </header>
            <main className="flex-1 py-6">{children}</main>
          </div>
        </div>
      </SessionContext.Provider>
    </ApiClientContext.Provider>
  );
}

function NavLink({
  active,
  href,
  children,
}: {
  active: boolean;
  href: string;
  children: React.ReactNode;
}) {
  return (
    <Link
      className={`rounded-full border px-4 py-2 font-medium transition ${
        active
          ? "border-stone-900/15 bg-white/85 text-stone-950"
          : "border-transparent text-stone-700 hover:border-stone-900/10 hover:bg-white/70 hover:text-stone-950"
      }`}
      href={href}
    >
      {children}
    </Link>
  );
}
