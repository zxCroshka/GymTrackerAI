"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Eye, EyeOff, LoaderCircle } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { useForm } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form-controls";
import { loginSchema, registerSchema, type LoginValues, type RegisterValues } from "@/features/auth/schemas";
import { apiErrorMessage } from "@/lib/api/errors";

type AuthFormProps =
  | { mode: "login"; onSubmit(values: LoginValues): Promise<void> }
  | { mode: "register"; onSubmit(values: RegisterValues): Promise<void> };

export function AuthForm(props: AuthFormProps) {
  const registerMode = props.mode === "register";
  const [showPassword, setShowPassword] = useState(false);
  const form = useForm<LoginValues & { confirmPassword?: string }>({
    resolver: zodResolver(registerMode ? registerSchema : loginSchema),
    defaultValues: { email: "", password: "", confirmPassword: "" },
  });

  async function submit(values: LoginValues & { confirmPassword?: string }) {
    form.clearErrors("root");
    try {
      if (props.mode === "register") {
        await props.onSubmit(values as RegisterValues);
      } else {
        await props.onSubmit(values);
      }
    } catch (error) {
      form.setError("root", { message: apiErrorMessage(error) });
    }
  }

  const passwordType = showPassword ? "text" : "password";
  return (
    <div>
      <div className="mb-8 lg:hidden"><p className="text-xl font-bold">GymTracker AI</p></div>
      <p className="text-sm font-semibold text-[var(--accent)]">{registerMode ? "Новый аккаунт" : "С возвращением"}</p>
      <h1 className="mt-2 text-3xl font-bold tracking-tight">{registerMode ? "Создать аккаунт" : "Войти"}</h1>
      <p className="mt-2 text-sm leading-6 text-[var(--muted)]">{registerMode ? "Начните вести тренировки и отслеживать прогресс." : "Продолжите работу со своей программой."}</p>

      <form className="mt-8 space-y-5" onSubmit={form.handleSubmit(submit)} noValidate>
        {form.formState.errors.root?.message && <div role="alert" className="rounded-xl bg-[var(--danger-soft)] px-4 py-3 text-sm text-[var(--danger)]">{form.formState.errors.root.message}</div>}
        <Field label="Email" htmlFor={`${props.mode}-email`} error={form.formState.errors.email?.message}>
          <Input id={`${props.mode}-email`} type="email" autoComplete="email" inputMode="email" aria-invalid={!!form.formState.errors.email} aria-describedby={form.formState.errors.email ? `${props.mode}-email-description` : undefined} {...form.register("email")} />
        </Field>
        <Field label="Пароль" htmlFor={`${props.mode}-password`} error={form.formState.errors.password?.message} hint={registerMode ? "От 12 до 128 символов" : undefined}>
          <div className="relative">
            <Input id={`${props.mode}-password`} type={passwordType} autoComplete={registerMode ? "new-password" : "current-password"} className="pr-12" aria-invalid={!!form.formState.errors.password} aria-describedby={`${props.mode}-password-description`} {...form.register("password")} />
            <button type="button" onClick={() => setShowPassword((value) => !value)} className="absolute inset-y-0 right-0 flex w-12 items-center justify-center rounded-r-xl text-[var(--muted)] hover:text-[var(--foreground)] focus-visible:outline-2" aria-label={showPassword ? "Скрыть пароль" : "Показать пароль"}>
              {showPassword ? <EyeOff aria-hidden="true" size={19} /> : <Eye aria-hidden="true" size={19} />}
            </button>
          </div>
        </Field>
        {registerMode && (
          <Field label="Повторите пароль" htmlFor="register-confirm-password" error={form.formState.errors.confirmPassword?.message}>
            <Input id="register-confirm-password" type={passwordType} autoComplete="new-password" aria-invalid={!!form.formState.errors.confirmPassword} aria-describedby={form.formState.errors.confirmPassword ? "register-confirm-password-description" : undefined} {...form.register("confirmPassword")} />
          </Field>
        )}
        <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
          {form.formState.isSubmitting && <LoaderCircle aria-hidden="true" size={18} className="animate-spin" />}
          {form.formState.isSubmitting ? "Отправляем…" : registerMode ? "Создать аккаунт" : "Войти"}
        </Button>
      </form>
      <p className="mt-6 text-center text-sm text-[var(--muted)]">
        {registerMode ? "Уже есть аккаунт?" : "Нет аккаунта?"}{" "}
        <Link href={registerMode ? "/login" : "/register"} className="font-semibold text-[var(--accent)] hover:underline">{registerMode ? "Войти" : "Зарегистрироваться"}</Link>
      </p>
    </div>
  );
}
