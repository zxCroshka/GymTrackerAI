import type { Metadata } from "next";
import { Suspense } from "react";

import { LoginView } from "@/features/auth/login-view";
import { Skeleton } from "@/components/ui/skeleton";

export const metadata: Metadata = { title: "Вход" };

export default function LoginPage() {
  return <Suspense fallback={<Skeleton className="h-96 w-full" />}><LoginView /></Suspense>;
}
