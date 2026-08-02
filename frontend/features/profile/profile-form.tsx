"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle, Save } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/form-controls";
import { profileFormSchema, type ProfileFormInput, type ProfilePatch } from "@/features/profile/schemas";
import type { Profile } from "@/features/profile/types";
import { apiErrorMessage } from "@/lib/api/errors";

function defaults(profile: Profile): ProfileFormInput {
  return {
    name: profile.name ?? "",
    sex: profile.sex ?? "",
    birth_date: profile.birth_date ?? "",
    height_cm: profile.height_cm?.toString() ?? "",
    goal: profile.goal ?? "",
    experience_level: profile.experience_level ?? "",
    training_frequency: profile.training_frequency?.toString() ?? "",
    timezone: profile.timezone,
    unit_system: profile.unit_system,
  };
}

export function ProfileForm({ profile, onSubmit }: { profile: Profile; onSubmit(values: ProfilePatch): Promise<void> }) {
  const form = useForm<ProfileFormInput, unknown, ProfilePatch>({ resolver: zodResolver(profileFormSchema), defaultValues: defaults(profile) });

  useEffect(() => form.reset(defaults(profile)), [form, profile]);

  async function submit(values: ProfilePatch) {
    form.clearErrors("root");
    try {
      await onSubmit(values);
    } catch (error) {
      form.setError("root", { message: apiErrorMessage(error) });
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} className="space-y-5" noValidate>
      {form.formState.errors.root?.message && <div role="alert" className="rounded-xl bg-[var(--danger-soft)] px-4 py-3 text-sm text-[var(--danger)]">{form.formState.errors.root.message}</div>}
      <div className="grid gap-5 sm:grid-cols-2">
        <Field label="Имя" htmlFor="profile-name" error={form.formState.errors.name?.message}><Input id="profile-name" autoComplete="name" aria-invalid={!!form.formState.errors.name} {...form.register("name")} /></Field>
        <Field label="Пол" htmlFor="profile-sex" error={form.formState.errors.sex?.message}><Select id="profile-sex" {...form.register("sex")}><option value="">Не указан</option><option value="male">Мужской</option><option value="female">Женский</option><option value="other">Другой</option><option value="prefer_not_to_say">Предпочитаю не указывать</option></Select></Field>
        <Field label="Дата рождения" htmlFor="profile-birth-date" error={form.formState.errors.birth_date?.message}><Input id="profile-birth-date" type="date" max={new Date().toISOString().slice(0, 10)} aria-invalid={!!form.formState.errors.birth_date} {...form.register("birth_date")} /></Field>
        <Field label="Рост, см" htmlFor="profile-height" error={form.formState.errors.height_cm?.message}><Input id="profile-height" type="number" min="50" max="300" step="0.1" inputMode="decimal" aria-invalid={!!form.formState.errors.height_cm} {...form.register("height_cm")} /></Field>
        <Field label="Цель" htmlFor="profile-goal" error={form.formState.errors.goal?.message}><Select id="profile-goal" {...form.register("goal")}><option value="">Не указана</option><option value="muscle_gain">Набор мышц</option><option value="weight_loss">Снижение массы</option><option value="recomposition">Рекомпозиция</option><option value="strength">Сила</option><option value="maintenance">Поддержание формы</option></Select></Field>
        <Field label="Уровень подготовки" htmlFor="profile-level" error={form.formState.errors.experience_level?.message}><Select id="profile-level" {...form.register("experience_level")}><option value="">Не указан</option><option value="beginner">Начинающий</option><option value="intermediate">Средний</option><option value="advanced">Продвинутый</option></Select></Field>
        <Field label="Тренировок в неделю" htmlFor="profile-frequency" error={form.formState.errors.training_frequency?.message}><Input id="profile-frequency" type="number" min="1" max="7" step="1" inputMode="numeric" aria-invalid={!!form.formState.errors.training_frequency} {...form.register("training_frequency")} /></Field>
        <Field label="Единицы измерения" htmlFor="profile-units" error={form.formState.errors.unit_system?.message}><Select id="profile-units" {...form.register("unit_system")}><option value="metric">Метрические — кг, см</option><option value="imperial">Имперские — фунты, дюймы</option></Select></Field>
      </div>
      <Field label="Часовой пояс" htmlFor="profile-timezone" error={form.formState.errors.timezone?.message} hint="Название IANA, например Europe/Moscow"><Input id="profile-timezone" autoComplete="off" list="common-timezones" aria-invalid={!!form.formState.errors.timezone} {...form.register("timezone")} /><datalist id="common-timezones"><option value="Europe/Moscow" /><option value="Europe/Berlin" /><option value="Asia/Almaty" /><option value="Asia/Tbilisi" /><option value="UTC" /></datalist></Field>
      <Button type="submit" disabled={form.formState.isSubmitting || !form.formState.isDirty}>
        {form.formState.isSubmitting ? <LoaderCircle aria-hidden="true" size={18} className="animate-spin" /> : <Save aria-hidden="true" size={18} />}
        {form.formState.isSubmitting ? "Сохраняем…" : "Сохранить профиль"}
      </Button>
    </form>
  );
}
