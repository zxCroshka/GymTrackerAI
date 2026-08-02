"use client";

import { Moon, Sun } from "lucide-react";

import { Button } from "@/components/ui/button";
import { resolveTheme, type Theme } from "@/lib/theme";

const storageKey = "gymtracker-theme";

export function ThemeToggle({ compact = false }: { compact?: boolean }) {
  function toggleTheme() {
    const current = resolveTheme(document.documentElement.dataset.theme ?? null, window.matchMedia("(prefers-color-scheme: dark)").matches);
    const next: Theme = current === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    window.localStorage.setItem(storageKey, next);
  }

  return (
    <Button type="button" variant="ghost" onClick={toggleTheme} aria-label="Сменить светлую или тёмную тему" title="Сменить тему" className={compact ? "size-11 px-0" : undefined}>
      <Sun aria-hidden="true" size={19} className="theme-icon-sun" />
      <Moon aria-hidden="true" size={19} className="theme-icon-moon" />
      {!compact && <span>Сменить тему</span>}
    </Button>
  );
}
