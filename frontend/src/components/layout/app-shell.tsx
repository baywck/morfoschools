// Morfosis — AppShell (dark shell + floating content card)
"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Sidebar, type NavItem } from "@/components/layout/sidebar";
import { Topbar } from "@/components/layout/topbar";

type AppShellProps = {
  children: ReactNode;
  navigation: NavItem[];
  title?: string;
  subtitle?: string;
  actions?: ReactNode;
  back?: string;
};

export function AppShell({ children, navigation, title, subtitle, actions, back }: AppShellProps) {
  return (
    <div className="h-screen overflow-hidden bg-[var(--shell)]">
      <Sidebar navigation={navigation} />
      <div className="relative flex h-screen flex-col md:pl-[var(--sidebar-width)]">
        {/* Dark shell header */}
        <Topbar />
        {/* Content — floating white card */}
        <div className="flex-1 min-h-0 p-0 md:pb-2.5 md:pr-2.5">
          <div className="flex h-full flex-col overflow-hidden rounded-none bg-[var(--background)] md:rounded-xl md:border md:border-[var(--border)] md:shadow-[0_20px_60px_rgba(0,0,0,0.12)]">
            <main className="min-h-0 flex-1 overflow-y-auto">
              {/* Inner sticky page header */}
              {title && (
                <div className="sticky top-0 z-20 bg-[var(--background)]/95 backdrop-blur-sm border-b border-[var(--border)]">
                  <div className="mx-auto w-full max-w-5xl flex items-center gap-3 px-5 py-3 md:px-7 lg:px-8">
                    {back && (
                      <Link
                        href={back}
                        className="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--border)] text-[var(--muted-foreground)] hover:text-[var(--foreground)] hover:border-[var(--border-strong)] transition-all shrink-0"
                      >
                        <ArrowLeft size={14} />
                      </Link>
                    )}
                    <div className="flex-1 min-w-0">
                      <h1 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight">{title}</h1>
                      {subtitle && <p className="text-[12px] text-[var(--muted-foreground)] mt-0.5">{subtitle}</p>}
                    </div>
                    {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
                  </div>
                </div>
              )}
              <div className="mx-auto w-full max-w-5xl px-5 py-6 md:px-7 md:py-7 lg:px-8">
                {children}
              </div>
            </main>
          </div>
        </div>
      </div>
    </div>
  );
}
