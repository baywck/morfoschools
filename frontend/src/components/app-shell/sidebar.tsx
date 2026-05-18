"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { GraduationCap } from "lucide-react";
import { cn } from "@/lib/cn";
import type { NavSection, NavItem } from "./types";

interface SidebarProps {
  sections: NavSection[];
  bottomItems: NavItem[];
  showPanel: boolean;
}

export function Sidebar({ sections, bottomItems, showPanel }: SidebarProps) {
  const pathname = usePathname();

  const allItems = sections.flatMap((s) => s.items);
  const activeItem = allItems.find(
    (item) => pathname === item.href || pathname.startsWith(item.href + "/")
  );

  return (
    <aside
      className={cn(
        "fixed inset-y-0 start-0 z-50 hidden lg:flex h-screen transition-all duration-200",
        showPanel ? "w-[var(--sidebar-width)]" : "w-[calc(var(--sidebar-collapsed-width)+1rem)]"
      )}
    >
      {/* Icon Strip */}
      <div className="flex w-[70px] flex-col items-center gap-4 bg-[var(--sidebar-icon-strip)] py-4 rounded-2xl m-2 min-h-[calc(100vh-1rem)] z-10">
        {/* Brand */}
        <div className="shrink-0 pt-1 pb-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-white/15 text-white">
            <GraduationCap className="h-5 w-5" />
          </div>
        </div>

        {/* Nav Icons */}
        <nav className="flex flex-1 flex-col items-center gap-1">
          {allItems.map((item) => {
            const Icon = item.icon;
            const isActive = pathname === item.href || pathname.startsWith(item.href + "/");
            return (
              <Link
                key={item.href}
                href={item.href}
                title={item.label}
                className={cn(
                  "flex h-9 w-9 items-center justify-center rounded-lg transition-all",
                  isActive
                    ? "bg-white/15 text-white"
                    : "text-white/40 hover:bg-white/8 hover:text-white/70"
                )}
              >
                <Icon className="h-5 w-5" />
              </Link>
            );
          })}
        </nav>

        {/* Bottom Icons */}
        <div className="shrink-0 flex flex-col items-center gap-1 pb-2">
          {bottomItems.map((item) => {
            const Icon = item.icon;
            const isActive = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                title={item.label}
                className={cn(
                  "flex h-9 w-9 items-center justify-center rounded-lg transition-all",
                  isActive
                    ? "bg-white/15 text-white"
                    : "text-white/40 hover:bg-white/8 hover:text-white/70"
                )}
              >
                <Icon className="h-5 w-5" />
              </Link>
            );
          })}
        </div>
      </div>

      {/* Menu Panel */}
      {showPanel && (
        <div className="grow border-e border-[var(--border)] bg-[var(--sidebar-menu)] overflow-y-auto">
          {/* Panel Title */}
          <div className="px-4 pt-5 pb-4">
            <h2 className="text-sm font-semibold text-[var(--foreground)]">
              {activeItem?.label || "Menu"}
            </h2>
          </div>

          {/* Sections */}
          <div className="px-2.5 space-y-5">
            {sections.map((section, idx) => (
              <div key={idx}>
                {section.title && (
                  <p className="px-2.5 pb-1.5 text-[11px] font-medium text-[var(--muted-foreground)]">
                    {section.title}
                  </p>
                )}
                <nav className="space-y-0.5">
                  {section.items.map((item) => {
                    const Icon = item.icon;
                    const isActive = pathname === item.href || pathname.startsWith(item.href + "/");
                    return (
                      <Link
                        key={item.href}
                        href={item.href}
                        className={cn(
                          "flex h-[34px] items-center gap-2.5 rounded-md px-2.5 text-[13px] transition-all",
                          isActive
                            ? "bg-[var(--card)] text-[var(--foreground)] font-medium shadow-sm"
                            : "text-[var(--muted-foreground)] hover:bg-[var(--card)]/70 hover:text-[var(--foreground)]"
                        )}
                      >
                        <Icon className="h-4 w-4" />
                        <span className="flex-1">{item.label}</span>
                        {item.badge && (
                          <span className="ml-auto rounded px-1.5 py-0.5 text-[10px] font-medium bg-[var(--danger-soft)] text-[var(--danger)]">
                            {item.badge}
                          </span>
                        )}
                      </Link>
                    );
                  })}
                </nav>
              </div>
            ))}
          </div>
        </div>
      )}
    </aside>
  );
}
