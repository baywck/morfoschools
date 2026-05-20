"use client";

import { cn } from "@/lib/cn";

/**
 * CoverageBadge — visual indicator of slot fill rate for an exam blueprint.
 * Used both inline (compact) and standalone (with full label).
 */
export function CoverageBadge({
  filled,
  total,
  strict = false,
  variant = "compact",
}: {
  filled: number;
  total: number;
  strict?: boolean;
  variant?: "compact" | "full";
}) {
  const pct = total > 0 ? filled / total : 0;
  const tone = pct >= 1
    ? "bg-[var(--success-soft)] text-[var(--success)] border-[var(--success)]/30"
    : pct >= 0.5
      ? "bg-[var(--warning-soft)] text-[var(--warning)] border-[var(--warning)]/30"
      : "bg-[var(--danger-soft)] text-[var(--danger)] border-[var(--danger)]/30";

  if (variant === "compact") {
    return (
      <span
        className={cn(
          "rounded-md border px-2 py-0.5 text-[10px] font-semibold",
          tone,
        )}
        title={`Coverage ${filled}/${total} slots filled`}
      >
        {filled}/{total} ({Math.round(pct * 100)}%)
      </span>
    );
  }

  return (
    <div className="space-y-1">
      <div className="flex items-baseline justify-between text-[12px]">
        <span className="font-semibold text-[var(--foreground)]">
          Coverage {filled}/{total}
        </span>
        <span
          className={cn(
            "font-semibold",
            pct >= 1
              ? "text-[var(--success)]"
              : pct >= 0.5
                ? "text-[var(--warning)]"
                : "text-[var(--danger)]",
          )}
        >
          {Math.round(pct * 100)}%
          {strict && pct < 1 ? " · strict" : ""}
        </span>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-[var(--muted)]">
        <div
          className={cn(
            "h-full transition-all",
            pct >= 1
              ? "bg-[var(--success)]"
              : pct >= 0.5
                ? "bg-[var(--warning)]"
                : "bg-[var(--danger)]",
          )}
          style={{ width: `${Math.min(100, Math.round(pct * 100))}%` }}
        />
      </div>
    </div>
  );
}
