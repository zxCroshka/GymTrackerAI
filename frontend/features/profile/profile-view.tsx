"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { MoonStar } from "lucide-react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth/auth-provider";
import { ErrorState } from "@/components/ui/async-state";
import { Card } from "@/components/ui/card";
import { PageSkeleton } from "@/components/ui/skeleton";
import { ImportProfileForm } from "@/features/profile/import-form";
import { ProfileForm } from "@/features/profile/profile-form";
import type { ProfileImport, ProfilePatch } from "@/features/profile/schemas";
import type { Profile, ProfileImportResult, ProfileResource } from "@/features/profile/types";
import { ApiError } from "@/lib/api/errors";

export function ProfileView() {
  const auth = useAuth();
  const queryClient = useQueryClient();
  const queryKey = ["profile", auth.session?.userId] as const;
  const profileQuery = useQuery({
    queryKey,
    queryFn: async (): Promise<ProfileResource> => {
      const response = await auth.api.request<Profile>("/api/v1/profile");
      if (!response.etag) throw new ApiError({ status: 200, code: "invalid_response", message: "Profile ETag is missing" });
      return { profile: response.data, etag: response.etag };
    },
  });

  const updateMutation = useMutation({
    mutationFn: async (values: ProfilePatch) => {
      const current = profileQuery.data;
      if (!current) throw new ApiError({ status: 0, code: "invalid_response", message: "Profile is unavailable" });
      const response = await auth.api.request<Profile>("/api/v1/profile", { method: "PATCH", headers: { "If-Match": current.etag }, json: values });
      if (!response.etag) throw new ApiError({ status: 200, code: "invalid_response", message: "Profile ETag is missing" });
      return { profile: response.data, etag: response.etag } satisfies ProfileResource;
    },
    onSuccess(resource) {
      queryClient.setQueryData(queryKey, resource);
      toast.success("Профиль обновлён");
    },
    onError(error) {
      if (error instanceof ApiError && error.status === 412) void queryClient.invalidateQueries({ queryKey });
    },
  });

  const importMutation = useMutation({
    mutationFn: async (values: ProfileImport) => {
      const current = profileQuery.data;
      if (!current) throw new ApiError({ status: 0, code: "invalid_response", message: "Profile is unavailable" });
      const response = await auth.api.request<ProfileImportResult>("/api/v1/profile/import", { method: "POST", headers: { "If-Match": current.etag }, json: values });
      if (!response.etag) throw new ApiError({ status: 200, code: "invalid_response", message: "Profile ETag is missing" });
      return { resource: { profile: response.data.profile, etag: response.etag } satisfies ProfileResource, measurementCreated: response.data.initial_measurement_id !== null };
    },
    onSuccess(result) {
      queryClient.setQueryData(queryKey, result.resource);
      toast.success("JSON-профиль импортирован", { description: result.measurementCreated ? "Начальное измерение также сохранено." : undefined });
    },
    onError(error) {
      if (error instanceof ApiError && error.status === 412) void queryClient.invalidateQueries({ queryKey });
    },
  });

  if (profileQuery.isPending) return <PageSkeleton />;
  if (profileQuery.isError) return <ErrorState title="Не удалось загрузить профиль" onRetry={() => void profileQuery.refetch()} />;
  const profile = profileQuery.data.profile;

  return (
    <div className="space-y-7">
      <header><p className="text-sm font-semibold text-[var(--accent)]">Аккаунт</p><h1 className="mt-1 text-3xl font-bold tracking-tight">Профиль</h1><p className="mt-2 text-sm text-[var(--muted)]">Настройте данные, которые влияют на календарь и отображение показателей.</p></header>
      <Card><h2 className="mb-6 text-xl font-bold">Персональные данные</h2><ProfileForm profile={profile} onSubmit={(values) => updateMutation.mutateAsync(values).then(() => undefined)} /></Card>
      {(profile.sleep_hours_average !== null || profile.notes.length > 0) && <Card><div className="flex items-center gap-3"><MoonStar aria-hidden="true" className="text-[var(--accent)]" /><h2 className="text-lg font-bold">Импортированные сведения</h2></div>{profile.sleep_hours_average !== null && <p className="mt-4 text-sm">Средняя продолжительность сна: <strong>{profile.sleep_hours_average} ч</strong></p>}{profile.notes.length > 0 && <ul className="mt-4 list-disc space-y-2 pl-5 text-sm text-[var(--muted)]">{profile.notes.map((note) => <li key={note}>{note}</li>)}</ul>}</Card>}
      <Card><h2 className="text-xl font-bold">Импорт JSON</h2><p className="mb-6 mt-2 text-sm leading-6 text-[var(--muted)]">Импорт выполняется транзакционно и может создать начальное измерение. Существующие значения изменятся только для переданных полей.</p><ImportProfileForm onSubmit={(values) => importMutation.mutateAsync(values).then(() => undefined)} /></Card>
    </div>
  );
}
