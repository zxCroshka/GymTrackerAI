"use client";

import { useEffect } from "react";

import { ErrorState } from "@/components/ui/async-state";

export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    // Error payloads are deliberately not logged in the browser because they
    // may contain sensitive API context. Production telemetry must redact it.
    void error.digest;
  }, [error]);
  return <main className="mx-auto flex min-h-screen max-w-lg items-center px-4"><ErrorState title="Страница временно недоступна" onRetry={reset} /></main>;
}
