"use client";

import { createContext, useContext, useCallback, useEffect, useState } from "react";
import { getMe, login, logout, type AuthSession, type LoginResponse } from "./auth";
import type { ApiResponse } from "./api-client";

interface AuthContextValue {
  session: AuthSession | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<ApiResponse<LoginResponse>>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<AuthSession | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    const res = await getMe();
    if (res.data) {
      setSession(res.data);
    } else {
      setSession(null);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleLogin = useCallback(
    async (email: string, password: string) => {
      const res = await login(email, password);
      if (res.data) {
        await refresh();
      }
      return res;
    },
    [refresh]
  );

  const handleLogout = useCallback(async () => {
    await logout();
    setSession(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        session,
        loading,
        login: handleLogin,
        logout: handleLogout,
        refresh,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
