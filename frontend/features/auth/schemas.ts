import { z } from "zod";

const email = z.string().trim().min(1, "Введите email").email("Введите корректный email").max(320, "Email слишком длинный").transform((value) => value.toLowerCase());
const password = z.string().min(12, "Минимум 12 символов").max(128, "Максимум 128 символов");

export const loginSchema = z.object({ email, password });
export const registerSchema = z
  .object({ email, password, confirmPassword: z.string().min(1, "Повторите пароль") })
  .refine((value) => value.password === value.confirmPassword, { path: ["confirmPassword"], message: "Пароли не совпадают" });

export type LoginValues = z.infer<typeof loginSchema>;
export type RegisterValues = z.infer<typeof registerSchema>;

export function safeNextPath(value: string | null): string {
  return value?.startsWith("/") && !value.startsWith("//") ? value : "/dashboard";
}
