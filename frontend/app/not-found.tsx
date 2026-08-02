import Link from "next/link";

import { Card } from "@/components/ui/card";

export default function NotFound() {
  return <main className="mx-auto flex min-h-screen max-w-lg items-center px-4"><Card><p className="text-sm font-semibold text-[var(--accent)]">404</p><h1 className="mt-2 text-2xl font-bold">Страница не найдена</h1><p className="mt-2 text-[var(--muted)]">Проверьте адрес или вернитесь на главную.</p><Link href="/dashboard" className="mt-6 inline-flex font-semibold text-[var(--accent)] hover:underline">На главную</Link></Card></main>;
}
