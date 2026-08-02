import { AlertCircle, RefreshCw } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export function ErrorState({ title = "Не удалось загрузить данные", description = "Повторите попытку. Если ошибка сохраняется, вернитесь позже.", onRetry }: { title?: string; description?: string; onRetry?: () => void }) {
  return (
    <Card className="flex flex-col items-start gap-4" role="alert">
      <span className="rounded-xl bg-[var(--danger-soft)] p-2.5 text-[var(--danger)]"><AlertCircle aria-hidden="true" size={22} /></span>
      <div><h2 className="font-semibold">{title}</h2><p className="mt-1 text-sm text-[var(--muted)]">{description}</p></div>
      {onRetry && <Button type="button" variant="secondary" onClick={onRetry}><RefreshCw aria-hidden="true" size={17} />Повторить</Button>}
    </Card>
  );
}
