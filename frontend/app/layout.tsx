import type { Metadata, Viewport } from "next";

import { Providers } from "@/components/providers";

import "./globals.css";

export const metadata: Metadata = {
  title: { default: "GymTracker AI", template: "%s · GymTracker AI" },
  description: "Тренировочные программы, дневник и понятная аналитика прогресса.",
};

export const viewport: Viewport = { width: "device-width", initialScale: 1, themeColor: "#16794b" };

const themeScript = `(() => { try { const saved = localStorage.getItem("gymtracker-theme"); const dark = matchMedia("(prefers-color-scheme: dark)").matches; document.documentElement.dataset.theme = saved === "light" || saved === "dark" ? saved : dark ? "dark" : "light"; } catch {} })();`;

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
