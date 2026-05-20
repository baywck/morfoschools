"use client";

import type { ReactNode } from "react";
import { Plus, ChevronLeft } from "lucide-react";
import { MobileActionsMenu } from "@/components/ui/mobile-actions-menu";

interface PageShellProps {
  title: string;
  subtitle?: string;
  /**
   * Back link rendered as a chevron button to the LEFT of the title,
   * matching the morfosis-studio reference. Provide an href to navigate
   * (uses anchor) or onClick for router.back().
   */
  back?: { href?: string; onClick?: () => void; label?: string };
  search?: {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
  };
  onAdd?: () => void;
  addLabel?: string;
  /**
   * Additional action buttons rendered to the RIGHT of the title row,
   * e.g. detail-page actions (toggle, publish, share). Renders inline
   * with the existing onAdd/search slots.
   */
  actions?: ReactNode;
  children: ReactNode;
}

export function PageShell({
  title,
  subtitle,
  back,
  search,
  onAdd,
  addLabel = "Add",
  actions,
  children,
}: PageShellProps) {
  const BackButton = back ? (
    back.href ? (
      <a
        href={back.href}
        aria-label={back.label || "Back"}
        className="shrink-0 flex h-8 w-8 items-center justify-center rounded-lg text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)] transition-colors"
      >
        <ChevronLeft size={16} />
      </a>
    ) : (
      <button
        type="button"
        onClick={back.onClick}
        aria-label={back.label || "Back"}
        className="shrink-0 flex h-8 w-8 items-center justify-center rounded-lg text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)] transition-colors"
      >
        <ChevronLeft size={16} />
      </button>
    )
  ) : null;

  return (
    <>
      {/* Sticky page header */}
      <div className="sticky top-0 z-20 bg-[var(--background)]/95 backdrop-blur-sm border-b border-[var(--border)]">
        {/* Main row */}
        <div className="mx-auto w-full max-w-5xl flex items-center gap-2 px-4 h-14 md:px-7 lg:px-8">
          {BackButton}
          <div className="flex-1 min-w-0">
            <h1 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight leading-tight truncate">{title}</h1>
            {subtitle && <p className="text-[11px] text-[var(--muted-foreground)] leading-tight truncate">{subtitle}</p>}
          </div>

          {/* Desktop: actions slot + search + full add button */}
          <div className="hidden md:flex items-center gap-2 shrink-0">
            {actions}
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

          {/* Mobile: actions collapse to a 3-dot dropdown so they don't
              compete with the title row, then the + add button stays
              alongside it (or alone if no actions). */}
          {actions && (
            <div className="md:hidden flex items-center gap-1.5 shrink-0">
              <MobileActionsMenu desktopClassName="hidden">
                {actions}
              </MobileActionsMenu>
            </div>
          )}
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
