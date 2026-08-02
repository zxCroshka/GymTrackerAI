import type { ButtonHTMLAttributes } from "react";

import { cn } from "@/lib/cn";

type Variant = "primary" | "secondary" | "ghost" | "danger";

export function Button({ className, variant = "primary", ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      className={cn(
        "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold transition-colors focus-visible:outline-2 disabled:cursor-not-allowed disabled:opacity-60",
        variant === "primary" && "bg-[var(--accent)] text-[var(--accent-foreground)] hover:bg-[var(--accent-hover)]",
        variant === "secondary" && "border border-[var(--border)] bg-[var(--surface)] hover:bg-[var(--surface-strong)]",
        variant === "ghost" && "hover:bg-[var(--surface-strong)]",
        variant === "danger" && "bg-[var(--danger-soft)] text-[var(--danger)] hover:opacity-85",
        className,
      )}
      {...props}
    />
  );
}
