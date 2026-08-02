"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { FileJson, LoaderCircle } from "lucide-react";
import { useForm } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Field, Textarea } from "@/components/ui/form-controls";
import { importFormSchema, type ImportFormInput, type ImportFormOutput, type ProfileImport } from "@/features/profile/schemas";
import { apiErrorMessage } from "@/lib/api/errors";

export function ImportProfileForm({ onSubmit }: { onSubmit(profile: ProfileImport): Promise<void> }) {
  const form = useForm<ImportFormInput, unknown, ImportFormOutput>({ resolver: zodResolver(importFormSchema), defaultValues: { json: "" } });

  async function submit(values: ImportFormOutput) {
    form.clearErrors("root");
    try {
      await onSubmit(values.profile);
      form.reset();
    } catch (error) {
      form.setError("root", { message: apiErrorMessage(error) });
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} className="space-y-4" noValidate>
      {form.formState.errors.root?.message && <div role="alert" className="rounded-xl bg-[var(--danger-soft)] px-4 py-3 text-sm text-[var(--danger)]">{form.formState.errors.root.message}</div>}
      <Field label="JSON-профиль" htmlFor="profile-import-json" error={form.formState.errors.json?.message} hint="Неизвестные поля будут отклонены до отправки.">
        <Textarea id="profile-import-json" spellCheck={false} className="min-h-64 font-mono text-sm" placeholder={'{\n  "name": "Руслан",\n  "goal": "recomposition"\n}'} aria-invalid={!!form.formState.errors.json} aria-describedby="profile-import-json-description" {...form.register("json")} />
      </Field>
      <Button type="submit" variant="secondary" disabled={form.formState.isSubmitting}>
        {form.formState.isSubmitting ? <LoaderCircle aria-hidden="true" size={18} className="animate-spin" /> : <FileJson aria-hidden="true" size={18} />}
        {form.formState.isSubmitting ? "Импортируем…" : "Импортировать JSON"}
      </Button>
    </form>
  );
}
