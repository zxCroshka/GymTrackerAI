"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { toast } from "sonner";

import { useAuth } from "@/components/auth/auth-provider";
import { AuthForm } from "@/features/auth/auth-form";
import { safeNextPath, type LoginValues } from "@/features/auth/schemas";

export function LoginView() {
  const auth = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();

  async function login(values: LoginValues) {
    await auth.login(values);
    toast.success("Вы вошли в аккаунт");
    router.replace(safeNextPath(searchParams.get("next")));
  }

  return <AuthForm mode="login" onSubmit={login} />;
}
