"use client";

import { Dumbbell, LayoutDashboard, LogOut, Settings, UserRound } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth/auth-provider";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { apiErrorMessage } from "@/lib/api/errors";
import { cn } from "@/lib/cn";

const navigation = [
  { href: "/dashboard", label: "Главная", icon: LayoutDashboard },
  { href: "/profile", label: "Профиль", icon: UserRound },
  { href: "/settings", label: "Настройки", icon: Settings },
];

function NavigationLink({ item, mobile = false }: { item: (typeof navigation)[number]; mobile?: boolean }) {
  const pathname = usePathname();
  const active = pathname === item.href;
  const Icon = item.icon;
  return (
    <Link
      href={item.href}
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex items-center rounded-xl text-sm font-semibold transition-colors focus-visible:outline-2",
        mobile ? "min-h-14 flex-1 flex-col justify-center gap-1 px-1 text-xs" : "min-h-11 gap-3 px-3.5",
        active ? "bg-[var(--accent-soft)] text-[var(--accent)]" : "text-[var(--muted)] hover:bg-[var(--surface-strong)] hover:text-[var(--foreground)]",
      )}
    >
      <Icon aria-hidden="true" size={mobile ? 20 : 19} />
      <span>{item.label}</span>
    </Link>
  );
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const auth = useAuth();
  const router = useRouter();
  const [loggingOut, setLoggingOut] = useState(false);

  async function logout() {
    setLoggingOut(true);
    try {
      await auth.logout();
      toast.success("Вы вышли из аккаунта");
      router.replace("/login");
    } catch (error) {
      toast.error(apiErrorMessage(error));
      router.replace("/login");
    } finally {
      setLoggingOut(false);
    }
  }

  return (
    <div className="min-h-screen md:pl-64">
      <a href="#main-content" className="fixed left-3 top-3 z-50 -translate-y-24 rounded-lg bg-[var(--accent)] px-4 py-2 text-[var(--accent-foreground)] transition-transform focus:translate-y-0">К основному содержимому</a>
      <aside className="fixed inset-y-0 left-0 hidden w-64 flex-col border-r border-[var(--border)] bg-[var(--surface)] p-4 md:flex">
        <Link href="/dashboard" className="flex min-h-12 items-center gap-3 rounded-xl px-2 font-bold tracking-tight focus-visible:outline-2">
          <span className="rounded-xl bg-[var(--accent)] p-2 text-[var(--accent-foreground)]"><Dumbbell aria-hidden="true" size={20} /></span>
          GymTracker AI
        </Link>
        <nav aria-label="Основная навигация" className="mt-7 space-y-1.5">{navigation.map((item) => <NavigationLink key={item.href} item={item} />)}</nav>
        <div className="mt-auto space-y-2 border-t border-[var(--border)] pt-4">
          <p className="truncate px-2 text-xs text-[var(--muted)]" title={auth.session?.email}>{auth.session?.email}</p>
          <ThemeToggle />
          <Button type="button" variant="ghost" className="w-full justify-start" onClick={logout} disabled={loggingOut}>
            <LogOut aria-hidden="true" size={18} />{loggingOut ? "Выходим…" : "Выйти"}
          </Button>
        </div>
      </aside>

      <header className="sticky top-0 z-30 flex min-h-16 items-center justify-between border-b border-[var(--border)] bg-[color:var(--surface)] px-4 md:hidden">
        <Link href="/dashboard" className="flex items-center gap-2 font-bold"><Dumbbell aria-hidden="true" size={21} className="text-[var(--accent)]" />GymTracker AI</Link>
        <ThemeToggle compact />
      </header>

      <main id="main-content" tabIndex={-1} className="mx-auto min-h-screen max-w-7xl px-4 pb-28 pt-6 sm:px-6 md:px-8 md:pb-10 md:pt-8">{children}</main>

      <nav aria-label="Мобильная навигация" className="safe-bottom fixed inset-x-0 bottom-0 z-40 flex border-t border-[var(--border)] bg-[var(--surface)] px-2 md:hidden">
        {navigation.map((item) => <NavigationLink key={item.href} item={item} mobile />)}
      </nav>
    </div>
  );
}
