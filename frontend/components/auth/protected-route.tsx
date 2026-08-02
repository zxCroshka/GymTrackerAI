"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

import { useAuth } from "@/components/auth/auth-provider";
import { ErrorState } from "@/components/ui/async-state";
import { PageSkeleton } from "@/components/ui/skeleton";

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const auth = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    if (auth.status === "unauthenticated") {
      const next = pathname && pathname !== "/dashboard" ? `?next=${encodeURIComponent(pathname)}` : "";
      router.replace(`/login${next}`);
    }
  }, [auth.status, pathname, router]);

  if (auth.status === "error") {
    return (
      <main className="mx-auto flex min-h-screen max-w-lg items-center px-4">
        <ErrorState title="Сервер временно недоступен" description="Не удалось проверить сессию. Проверьте подключение и повторите попытку." onRetry={() => void auth.retryRestore()} />
      </main>
    );
  }
  if (auth.status !== "authenticated") {
    return <main className="mx-auto min-h-screen max-w-7xl px-4 py-8"><PageSkeleton /></main>;
  }
  return children;
}
