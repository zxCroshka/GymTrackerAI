"use client";

import { useQuery } from "@tanstack/react-query";
import { CalendarClock, Dumbbell, Flame, Scale, TrendingUp } from "lucide-react";

import { useAuth } from "@/components/auth/auth-provider";
import { ErrorState } from "@/components/ui/async-state";
import { Card } from "@/components/ui/card";
import { PageSkeleton } from "@/components/ui/skeleton";
import type { ProgressDashboard } from "@/features/dashboard/types";

const number = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 });

function metric(value: number | null, suffix: string): string {
  return value === null ? "Недостаточно данных" : `${number.format(value)} ${suffix}`;
}

function change(value: number | null): string {
  if (value === null) return "Нет значения для сравнения";
  return `${value > 0 ? "+" : ""}${number.format(value)} кг за 7 дней`;
}

const recordLabels: Record<ProgressDashboard["new_achievements"][number]["record_type"], string> = {
  max_weight: "Максимальный вес",
  max_reps: "Максимум повторений",
  max_set_volume: "Объём подхода",
  estimated_1rm: "Расчётный 1ПМ",
};

export function DashboardView() {
  const auth = useAuth();
  const query = useQuery({
    queryKey: ["progress", "dashboard", auth.session?.userId],
    queryFn: async () => (await auth.api.request<ProgressDashboard>("/api/v1/progress/dashboard")).data,
  });

  if (query.isPending) return <PageSkeleton />;
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />;
  const dashboard = query.data;

  const cards = [
    { label: "Текущая масса", value: metric(dashboard.weight.current_kg, "кг"), detail: change(dashboard.weight.change_7d_kg), icon: Scale },
    { label: "Тренировок на неделе", value: number.format(dashboard.workouts_this_week), detail: "Завершённые тренировки", icon: Dumbbell },
    { label: "Недельный объём", value: metric(dashboard.weekly_volume_kg, "кг"), detail: `Всего: ${metric(dashboard.total_volume_kg, "кг")}`, icon: TrendingUp },
    { label: "Тренировочная серия", value: `${dashboard.training_streak_weeks} нед.`, detail: "Последовательные активные недели", icon: Flame },
  ];

  return (
    <div className="space-y-7">
      <header><p className="text-sm font-semibold text-[var(--accent)]">Обзор</p><h1 className="mt-1 text-3xl font-bold tracking-tight">Ваш прогресс</h1><p className="mt-2 text-sm text-[var(--muted)]">Данные на {new Intl.DateTimeFormat("ru-RU", { dateStyle: "long", timeZone: dashboard.timezone }).format(new Date(dashboard.as_of))}</p></header>
      <section aria-label="Ключевые показатели" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {cards.map(({ label, value, detail, icon: Icon }) => (
          <Card key={label}><span className="inline-flex rounded-xl bg-[var(--accent-soft)] p-2.5 text-[var(--accent)]"><Icon aria-hidden="true" size={20} /></span><p className="mt-5 text-sm text-[var(--muted)]">{label}</p><p className="mt-1 text-xl font-bold">{value}</p><p className="mt-2 text-xs leading-5 text-[var(--muted)]">{detail}</p></Card>
        ))}
      </section>
      <div className="grid gap-5 xl:grid-cols-2">
        <Card>
          <div className="flex items-center gap-3"><CalendarClock aria-hidden="true" className="text-[var(--accent)]" /><h2 className="text-lg font-bold">Ближайшая тренировка</h2></div>
          {dashboard.next_planned_workout ? <div className="mt-5"><p className="font-semibold">{dashboard.next_planned_workout.name}</p><p className="mt-1 text-sm text-[var(--muted)]">{new Intl.DateTimeFormat("ru-RU", { dateStyle: "long", timeStyle: "short", timeZone: dashboard.timezone }).format(new Date(dashboard.next_planned_workout.scheduled_at))}</p></div> : <p className="mt-5 text-sm text-[var(--muted)]">Плановая тренировка пока не назначена.</p>}
        </Card>
        <Card>
          <h2 className="text-lg font-bold">Новые достижения</h2>
          {dashboard.new_achievements.length ? <ul className="mt-4 divide-y divide-[var(--border)]">{dashboard.new_achievements.slice(0, 5).map((record) => <li key={record.id} className="py-3 first:pt-0"><p className="font-semibold">{record.exercise_name}</p><p className="mt-1 text-sm text-[var(--muted)]">{recordLabels[record.record_type]}: {number.format(record.value)} {record.unit === "repetitions" ? "повт." : record.unit === "kg_repetitions" ? "кг·повт." : "кг"}</p></li>)}</ul> : <p className="mt-4 text-sm text-[var(--muted)]">На этой неделе новых рекордов пока нет.</p>}
        </Card>
      </div>
    </div>
  );
}
