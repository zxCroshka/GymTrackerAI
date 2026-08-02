"use client";

import { LogOut, Palette, ShieldCheck } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth/auth-provider";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { apiErrorMessage } from "@/lib/api/errors";

export function SettingsView() {
  const auth = useAuth();
  const router = useRouter();
  const [loggingOut, setLoggingOut] = useState(false);

  async function logout() {
    setLoggingOut(true);
    try {
      await auth.logout();
      toast.success("Вы вышли из аккаунта");
    } catch (error) {
      toast.error(apiErrorMessage(error));
    } finally {
      router.replace("/login");
      setLoggingOut(false);
    }
  }

  return (
    <div className="space-y-7">
      <header><p className="text-sm font-semibold text-[var(--accent)]">Приложение</p><h1 className="mt-1 text-3xl font-bold tracking-tight">Настройки</h1></header>
      <div className="grid gap-5 lg:grid-cols-2">
        <Card><div className="flex items-center gap-3"><Palette aria-hidden="true" className="text-[var(--accent)]" /><h2 className="text-lg font-bold">Оформление</h2></div><p className="mb-4 mt-3 text-sm leading-6 text-[var(--muted)]">Выбранная тема сохраняется только в этом браузере.</p><ThemeToggle /></Card>
        <Card><div className="flex items-center gap-3"><ShieldCheck aria-hidden="true" className="text-[var(--accent)]" /><h2 className="text-lg font-bold">Текущая сессия</h2></div><p className="mt-3 break-all text-sm text-[var(--muted)]">{auth.session?.email}</p><p className="mt-2 text-xs leading-5 text-[var(--muted)]">Access token хранится только в памяти вкладки. Refresh token защищён HttpOnly cookie и недоступен JavaScript.</p><Button type="button" variant="danger" className="mt-5" onClick={logout} disabled={loggingOut}><LogOut aria-hidden="true" size={18} />{loggingOut ? "Выходим…" : "Выйти из аккаунта"}</Button></Card>
      </div>
    </div>
  );
}
