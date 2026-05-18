// Morfosis — AppShell (dark shell + floating content card)
"use client";

import type { ReactNode } from "react";
import { Sidebar, type NavItem } from "@/components/layout/sidebar";
import { Topbar } from "@/components/layout/topbar";
import { MobileNav } from "@/components/layout/mobile-nav";

type AppShellProps = {
  children: ReactNode;
  navigation: NavItem[];
};

export function AppShell({ children, navigation }: AppShellProps) {
  return (
    <div className="h-screen overflow-hidden bg-[var(--shell)]">
      {/* Desktop sidebar */}
      <Sidebar navigation={navigation} />

      <div className="relative flex h-screen flex-col md:pl-[var(--sidebar-width)]">
        {/* Topbar — in dark shell */}
        <Topbar />

        {/* Content — floating white card */}
        <div className="flex-1 min-h-0 px-2 pb-[4.5rem] md:p-0 md:pb-2.5 md:pr-2.5">
          <div className="relative flex h-full flex-col overflow-hidden bg-[var(--background)] rounded-2xl md:rounded-xl md:border md:border-[var(--border)] md:shadow-[0_20px_60px_rgba(0,0,0,0.12)]">
            <main className="min-h-0 flex-1 overflow-y-auto">
              {children}
            </main>
          </div>
        </div>
      </div>

      {/* Mobile bottom nav */}
      <MobileNav navigation={navigation} />
    </div>
  );
}
