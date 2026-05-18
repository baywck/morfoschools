"use client";

import * as React from "react";
import { cn } from "@/lib/cn";

interface FieldShellProps {
  id: string;
  label: string;
  value?: string | number | readonly string[];
  focused?: boolean;
  error?: string;
  hint?: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
}

export function FieldShell({ id, label, value, focused, error, hint, icon, children }: FieldShellProps) {
  const hasValue = value !== undefined && value !== null && value !== "";
  const isFloating = focused || hasValue;

  return (
    <div>
      <div
        className={cn(
          "relative flex rounded-lg border bg-[var(--card)] transition",
          icon ? "items-start" : "items-center",
          focused && !error && "border-[var(--primary)] ring-2 ring-[var(--primary)]/20",
          error && "border-[var(--danger)]/40 ring-2 ring-[var(--danger)]/10",
          !focused && !error && "border-[var(--border)]"
        )}
      >
        {icon && (
          <span className="ml-2.5 mr-3 mt-2 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-[var(--border)] bg-[var(--muted)] text-[var(--muted-foreground)]">
            {icon}
          </span>
        )}
        <div className="relative flex-1">
          <label
            htmlFor={id}
            className={cn(
              "pointer-events-none absolute transition-all duration-150",
              error ? "text-[var(--danger)]/70" : "text-[var(--muted-foreground)]",
              icon
                ? isFloating
                  ? "left-0 top-1.5 text-[10px] font-medium"
                  : "left-0 top-1/2 -translate-y-1/2 text-sm"
                : isFloating
                  ? "left-3 top-1.5 text-[10px] font-medium"
                  : "left-3 top-1/2 -translate-y-1/2 text-sm"
            )}
          >
            {label}
          </label>
          {children}
        </div>
      </div>
      {error && (
        <p className="mt-1 text-[11px] font-medium text-[var(--danger)]" role="alert">
          {error}
        </p>
      )}
      {hint && !error && (
        <p className="mt-1 text-[11px] text-[var(--muted-foreground)]">{hint}</p>
      )}
    </div>
  );
}
