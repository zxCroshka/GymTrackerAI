"use client";

import { useRouter } from "next/navigation";
import { toast } from "sonner";

import { useAuth } from "@/components/auth/auth-provider";
import { AuthForm } from "@/features/auth/auth-form";
import type { RegisterValues } from "@/features/auth/schemas";

export function RegisterView() {
  const auth = useAuth();
  const router = useRouter();

  async function register(values: RegisterValues) {
    await auth.register({ email: values.email, password: values.password });
    toast.success("Аккаунт создан");
    router.replace("/dashboard");
  }

  return <AuthForm mode="register" onSubmit={register} />;
}
