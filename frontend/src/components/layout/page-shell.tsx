"use client";

import type { ReactNode } from "react";
import { Plus } from "lucide-react";

interface PageShellProps {
  title: string;
  subtitle?: string;
  search?: {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
  };
  onAdd?: () => void;
  addLabel?: string;
  children: ReactNode;
}

export function PageShell({ title, subtitle, search, onAdd, addLabel = "Add", children }: PageShellProps) {
  return (
    <>
      {/* Sticky page header */}
      <div className="sticky top-0 z-20 bg-[var(--background)]/95 backdrop-blur-sm border-b border-[var(--border)]">
        {/* Main row */}
        <div className="mx-auto w-full max-w-5xl flex items-center gap-3 px-4 h-14 md:px-7 lg:px-8">
          <div className="flex-1 min-w-0">
            <h1 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight leading-tight">{title}</h1>
            {subtitle && <p className="text-[11px] text-[var(--muted-foreground)] leading-tight">{subtitle}</p>}
          </div>

          {/* Desktop: search + full add button */}
          <div className="hidden md:flex items-center gap-2 shrink-0">
            {search && (
              <div className="flex h-8 items-center rounded-lg border border-[var(--border)] bg-[var(--background)] px-2.5 gap-2 w-44">
                <svg className="shrink-0 h-3.5 w-3.5 text-[var(--muted-foreground)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <circle cx="11" cy="11" r="8" /><path d="m21 21-4.3-4.3" />
                </svg>
                <input
                  type="text"
                  value={search.value}
                  onChange={(e) => search.onChange(e.target.value)}
                  placeholder={search.placeholder || "Search..."}
                  className="w-full bg-transparent text-[12px] outline-none placeholder:text-[var(--muted-foreground)] text-[var(--foreground)]"
                />
              </div>
            )}
            {onAdd && (
              <button
                onClick={onAdd}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] transition-all"
              >
                <Plus size={14} /> {addLabel}
              </button>
            )}
          </div>

          {/* Mobile: + icon button only */}
          {onAdd && (
            <button
              onClick={onAdd}
              className="md:hidden flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--primary)] text-[var(--primary-foreground)] shadow-sm active:scale-[0.97] transition-all"
            >
              <Plus size={16} />
            </button>
          )}
        </div>

        {/* Mobile search row */}
        {search && (
          <div className="md:hidden px-4 pb-3">
            <div className="flex h-8 items-center rounded-lg border border-[var(--border)] bg-[var(--background)] px-2.5 gap-2">
              <svg className="shrink-0 h-3.5 w-3.5 text-[var(--muted-foreground)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <circle cx="11" cy="11" r="8" /><path d="m21 21-4.3-4.3" />
              </svg>
              <input
                type="text"
                value={search.value}
                onChange={(e) => search.onChange(e.target.value)}
                placeholder={search.placeholder || "Search..."}
                className="w-full bg-transparent text-[12px] outline-none placeholder:text-[var(--muted-foreground)] text-[var(--foreground)]"
              />
            </div>
          </div>
        )}
      </div>

      {/* Content */}
      <div className="mx-auto w-full max-w-5xl px-4 py-5 md:px-7 md:py-7 lg:px-8">
        {children}
      </div>
    </>
  );
}
