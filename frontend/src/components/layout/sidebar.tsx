// Morfosis — Sidebar (66px dark icon strip)
"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/cn";
import type { LucideIcon } from "lucide-react";

export type NavItem = {
  label: string;
  href: string;
  icon: LucideIcon;
};

interface SidebarProps {
  navigation: NavItem[];
  brand?: React.ReactNode;
}

export function Sidebar({ navigation, brand }: SidebarProps) {
  const pathname = usePathname();
  const [pendingHref, setPendingHref] = useState<string | null>(null);

  // Clear pending when pathname changes (page loaded)
  useEffect(() => {
    setPendingHref(null);
  }, [pathname]);

  return (
    <aside className="fixed inset-y-0 left-0 z-40 hidden md:flex w-[var(--sidebar-width)] flex-col items-center bg-[var(--shell)] py-4">
      {/* Brand */}
      <div className="flex w-full justify-center mb-6">
        <Link href="/app" aria-label="Home">
          {brand || (
            <img src="/logo.png" alt="Morfoschools" className="h-7 w-7" />
          )}
        </Link>
      </div>

      {/* Navigation */}
      <nav className="flex flex-1 flex-col items-center w-full gap-1.5">
        {navigation.map((item) => {
          const Icon = item.icon;
          const isActive =
            item.href === "/app"
              ? pathname === item.href
              : pathname.startsWith(item.href);
          const isPending = pendingHref === item.href;

          return (
            <Link
              key={item.href}
              href={item.href}
              onClick={() => { if (!isActive) setPendingHref(item.href); }}
              className={cn(
                "group relative flex h-[42px] w-[42px] items-center justify-center rounded-[11px] transition-all duration-200",
                isActive
                  ? "bg-[var(--shell-active)] shadow-[inset_0_0_0_1px_rgba(0,0,0,0.04)]"
                  : "hover:bg-[var(--shell-hover)]"
              )}
              aria-label={item.label}
              title={item.label}
            >
              {isPending ? (
                <Loader2
                  className="h-[18px] w-[18px] shrink-0 animate-spin text-[var(--shell-foreground)]"
                />
              ) : (
                <Icon
                  className={cn(
                    "h-[18px] w-[18px] shrink-0 transition-colors duration-200",
                    isActive
                      ? "text-[var(--shell-foreground)] stroke-[2.25]"
                      : "text-[var(--shell-muted)] group-hover:text-[var(--shell-foreground)] stroke-[2]"
                  )}
                />
              )}

              {/* Tooltip */}
              <span className="pointer-events-none absolute left-[calc(100%+12px)] top-1/2 -translate-y-1/2 rounded-lg border border-[var(--shell-border,var(--border-strong))] bg-[var(--shell-elevated,var(--card))] px-2.5 py-1 text-[11px] font-semibold text-[var(--shell-foreground)] opacity-0 shadow-lg transition-all duration-200 group-hover:translate-x-1 group-hover:opacity-100 whitespace-nowrap z-50">
                {item.label}
              </span>
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
