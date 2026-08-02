"use client";

import { useEffect } from "react";

import { resolveTheme, type Theme } from "@/lib/theme";

const storageKey = "gymtracker-theme";

export function ThemeToggle() {
  useEffect(() => {
    const selected = resolveTheme(
      window.localStorage.getItem(storageKey),
      window.matchMedia("(prefers-color-scheme: dark)").matches,
    );
    document.documentElement.dataset.theme = selected;
  }, []);

  function toggleTheme() {
    const current = resolveTheme(
      document.documentElement.dataset.theme ?? null,
      window.matchMedia("(prefers-color-scheme: dark)").matches,
    );
    const next: Theme = current === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    window.localStorage.setItem(storageKey, next);
  }

  return (
    <button
      type="button"
      onClick={toggleTheme}
      className="rounded-full border border-[var(--border)] px-4 py-2 text-sm font-medium transition-colors hover:bg-[var(--background)] focus-visible:outline-2"
      aria-label="Переключить светлую и тёмную тему"
    >
      Сменить тему
    </button>
  );
}
