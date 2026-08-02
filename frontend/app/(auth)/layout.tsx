import { Dumbbell } from "lucide-react";
import Link from "next/link";

import { AuthPageGuard } from "@/components/auth/auth-page-guard";
import { ThemeToggle } from "@/components/theme-toggle";

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthPageGuard>
      <main className="relative grid min-h-screen lg:grid-cols-[1.05fr_0.95fr]">
        <div className="absolute right-4 top-4 z-10"><ThemeToggle compact /></div>
        <section className="hidden bg-[var(--accent)] p-12 text-[var(--accent-foreground)] lg:flex lg:flex-col lg:justify-between">
          <Link href="/" className="flex items-center gap-3 text-xl font-bold"><Dumbbell aria-hidden="true" />GymTracker AI</Link>
          <div className="max-w-xl"><p className="text-sm font-semibold uppercase tracking-[0.18em] opacity-80">Тренируйся осознанно</p><h1 className="mt-4 text-5xl font-bold leading-tight">Программа, дневник и прогресс — в одной системе.</h1><p className="mt-6 text-lg leading-8 opacity-85">Фиксируйте тренировки и принимайте решения на основе собственных данных.</p></div>
          <p className="text-sm opacity-75">Ваши данные доступны только вашему аккаунту.</p>
        </section>
        <section className="flex items-center justify-center px-4 py-20 sm:px-8"><div className="w-full max-w-md">{children}</div></section>
      </main>
    </AuthPageGuard>
  );
}
