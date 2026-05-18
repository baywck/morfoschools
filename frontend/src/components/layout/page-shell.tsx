"use client";

import { useState, useRef, useEffect } from "react";
import type { ReactNode } from "react";
import { MoreVertical } from "lucide-react";
import { cn } from "@/lib/cn";

interface PageAction {
  label: string;
  icon?: ReactNode;
  onClick: () => void;
  variant?: "default" | "danger";
}

interface PageShellProps {
  title: string;
  subtitle?: string;
  actions?: PageAction[];
  search?: {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
  };
  children: ReactNode;
}

export function PageShell({ title, subtitle, actions, search, children }: PageShellProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  return (
    <>
      {/* Sticky page header */}
      <div className="sticky top-0 z-20 bg-[var(--background)]/95 backdrop-blur-sm border-b border-[var(--border)]">
        {/* Main row: title + actions */}
        <div className="mx-auto w-full max-w-5xl flex items-center gap-3 px-4 h-14 md:px-7 lg:px-8">
          <div className="flex-1 min-w-0">
            <h1 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight leading-tight">{title}</h1>
            {subtitle && <p className="text-[11px] text-[var(--muted-foreground)] leading-tight">{subtitle}</p>}
          </div>

          {/* Desktop: search + buttons */}
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
            {actions?.map((action, i) => (
              <button
                key={i}
                onClick={action.onClick}
                className={cn(
                  "inline-flex h-8 items-center gap-1.5 rounded-lg px-3 text-[12px] font-semibold transition-all active:scale-[0.97]",
                  action.variant === "danger"
                    ? "bg-[var(--danger)] text-white shadow-sm hover:opacity-90"
                    : "bg-[var(--primary)] text-[var(--primary-foreground)] shadow-sm hover:opacity-90"
                )}
              >
                {action.icon}
                {action.label}
              </button>
            ))}
          </div>

          {/* Mobile: 3-dot menu */}
          {(actions?.length || search) && (
            <div className="relative md:hidden" ref={menuRef}>
              <button
                onClick={() => setMenuOpen((v) => !v)}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)] transition-colors"
              >
                <MoreVertical size={16} />
              </button>

              {menuOpen && (
                <div className="absolute right-0 top-full mt-1.5 w-48 rounded-xl border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg z-50">
                  {actions?.map((action, i) => (
                    <button
                      key={i}
                      onClick={() => { action.onClick(); setMenuOpen(false); }}
                      className={cn(
                        "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-[12px] font-medium transition-colors",
                        action.variant === "danger"
                          ? "text-[var(--danger)] hover:bg-[var(--danger-soft)]"
                          : "text-[var(--foreground)] hover:bg-[var(--muted)]"
                      )}
                    >
                      {action.icon}
                      {action.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Mobile search row (below title) */}
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
