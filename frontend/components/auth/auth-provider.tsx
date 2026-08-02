"use client";

import { useQueryClient } from "@tanstack/react-query";
import { createContext, useContext, useEffect, useMemo, useRef, useSyncExternalStore } from "react";
import { toast } from "sonner";

import { authStore, serverAuthSnapshot, type AuthSnapshot, type Credentials } from "@/lib/auth/store";

interface AuthContextValue extends AuthSnapshot {
  api: typeof authStore.api;
  login(credentials: Credentials): Promise<void>;
  register(credentials: Credentials): Promise<void>;
  logout(): Promise<void>;
  retryRestore(): Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();
  const snapshot = useSyncExternalStore(authStore.subscribe, authStore.getSnapshot, () => serverAuthSnapshot);
  const previous = useRef(snapshot);

  useEffect(() => {
    void authStore.restore();
  }, []);

  useEffect(() => {
    const old = previous.current;
    if (snapshot.status === "unauthenticated" && snapshot.reason === "expired" && old.status === "authenticated") {
      toast.error("Сессия истекла", { description: "Войдите снова, чтобы продолжить." });
    }
    if (old.session?.userId && old.session.userId !== snapshot.session?.userId) queryClient.clear();
    previous.current = snapshot;
  }, [queryClient, snapshot]);

  useEffect(() => {
    if (snapshot.status !== "authenticated" || !snapshot.session) return;
    const expiresAt = Date.parse(snapshot.session.accessExpiresAt);
    const delay = Math.max(1_000, expiresAt - Date.now() - 60_000);
    const timer = window.setTimeout(() => {
      void authStore.refresh().catch(() => {
        toast.error("Не удалось обновить сессию", { description: "Проверим её снова при следующем запросе." });
      });
    }, delay);
    return () => window.clearTimeout(timer);
  }, [snapshot]);

  const value = useMemo<AuthContextValue>(
    () => ({
      ...snapshot,
      api: authStore.api,
      login: (credentials) => authStore.login(credentials),
      register: (credentials) => authStore.register(credentials),
      logout: () => authStore.logout(),
      retryRestore: () => authStore.restore(),
    }),
    [snapshot],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
