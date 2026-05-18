"use client";

import { useState } from "react";
import { Sidebar } from "./sidebar";
import { Header } from "./header";
import { MobileNav } from "./mobile-nav";
import type { NavSection, NavItem } from "./types";

interface AppShellProps {
  sections: NavSection[];
  bottomItems?: NavItem[];
  showPanel?: boolean;
  children: React.ReactNode;
}

export function AppShell({ sections, bottomItems = [], showPanel: initialShowPanel = true, children }: AppShellProps) {
  const [showPanel, setShowPanel] = useState(initialShowPanel);
  const [theme, setTheme] = useState<"light" | "dark">("light");

  const toggleTheme = () => {
    const next = theme === "light" ? "dark" : "light";
    setTheme(next);
    document.documentElement.setAttribute("data-theme", next);
  };

  const sidebarOffset = showPanel
    ? "lg:ps-[var(--sidebar-width)]"
    : "lg:ps-[calc(var(--sidebar-collapsed-width)+1rem)]";

  const shellBorder = showPanel ? "" : "lg:border-s lg:border-[var(--border)]";

  return (
    <div className="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
      {/* Header */}
      <div className={`fixed inset-x-0 top-0 z-30 ${sidebarOffset} ${shellBorder}`}>
        <Header
          showPanel={showPanel}
          onTogglePanel={() => setShowPanel((v) => !v)}
          theme={theme}
          onToggleTheme={toggleTheme}
        />
      </div>

      {/* Sidebar */}
      <Sidebar sections={sections} bottomItems={bottomItems} showPanel={showPanel} />

      {/* Mobile Nav */}
      <MobileNav sections={sections} />

      {/* Main Content */}
      <main className={`pt-[var(--header-height)] pb-16 lg:pb-0 ${sidebarOffset} ${shellBorder}`}>
        <div className="space-y-5 p-5">{children}</div>
      </main>
    </div>
  );
}
