"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { useAuth } from "@/components/auth/auth-provider";
import { Skeleton } from "@/components/ui/skeleton";

export function AuthPageGuard({ children }: { children: React.ReactNode }) {
  const auth = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (auth.status === "authenticated") router.replace("/dashboard");
  }, [auth.status, router]);

  if (auth.status === "bootstrapping") {
    return <div className="w-full max-w-md space-y-4" role="status" aria-label="Проверка сессии"><Skeleton className="h-12" /><Skeleton className="h-96" /></div>;
  }
  return children;
}
