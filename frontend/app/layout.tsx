import type { Metadata } from "next";

import { ThemeToggle } from "@/components/theme-toggle";

import "./globals.css";

export const metadata: Metadata = {
  title: "GymTracker AI",
  description: "A planned strength-training tracker with a user-controlled AI coach.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru" suppressHydrationWarning>
      <body>
        <header className="border-b border-[var(--border)] bg-[var(--surface)]">
          <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
            <span className="text-lg font-semibold tracking-tight">GymTracker AI</span>
            <ThemeToggle />
          </div>
        </header>
        {children}
      </body>
    </html>
  );
}
