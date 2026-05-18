"use client";

import type { ReactNode } from "react";

interface PageShellProps {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
  children: ReactNode;
}

export function PageShell({ title, subtitle, actions, children }: PageShellProps) {
  return (
    <>
      {/* Sticky page header */}
      <div className="sticky top-0 z-20 bg-[var(--background)]/95 backdrop-blur-sm border-b border-[var(--border)]">
        <div className="mx-auto w-full max-w-5xl flex items-center gap-3 px-4 py-3 md:px-7 lg:px-8">
          <div className="flex-1 min-w-0">
            <h1 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight">{title}</h1>
            {subtitle && <p className="text-[12px] text-[var(--muted-foreground)] mt-0.5">{subtitle}</p>}
          </div>
          {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
        </div>
      </div>
      {/* Content */}
      <div className="mx-auto w-full max-w-5xl px-4 py-5 md:px-7 md:py-7 lg:px-8">
        {children}
      </div>
    </>
  );
}
