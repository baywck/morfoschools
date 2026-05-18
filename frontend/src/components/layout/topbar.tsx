// Morfosis — Topbar (dark shell area: breadcrumb + search + theme)
"use client";

import { Search, Moon, Sun } from "lucide-react";
import { useTheme } from "@/lib/use-theme";

export function Topbar() {
  const { dark, toggle } = useTheme();

  return (
    <header className="flex h-[var(--header-height)] items-center gap-3 pl-1 pr-5">
      {/* Left — title/breadcrumb area */}
      <div className="min-w-0 flex-1">
        <span className="text-[13px] font-semibold text-[var(--shell-foreground)]">
          Morfoschools
        </span>
      </div>

      {/* Actions */}
      <div className="flex items-center gap-1.5">
        {/* Search */}
        <button className="flex h-8 items-center gap-2 rounded-lg border border-white/10 bg-white/[0.06] px-3 text-[12px] text-[var(--shell-muted)] hover:bg-white/[0.1] hover:text-[var(--shell-foreground)] transition-all">
          <Search size={13} strokeWidth={2} />
          <span className="hidden sm:inline">Search...</span>
          <kbd className="hidden sm:inline-flex h-[18px] items-center rounded border border-white/10 bg-white/[0.06] px-1 text-[10px] font-medium ml-2">
            ⌘K
          </kbd>
        </button>

        {/* Theme toggle */}
        <button
          onClick={toggle}
          className="flex h-8 w-8 items-center justify-center rounded-lg border border-white/10 bg-white/[0.06] text-[var(--shell-muted)] hover:bg-white/[0.1] hover:text-[var(--shell-foreground)] transition-all"
          aria-label={dark ? "Switch to light mode" : "Switch to dark mode"}
        >
          {dark ? <Sun size={14} strokeWidth={2} /> : <Moon size={14} strokeWidth={2} />}
        </button>
      </div>
    </header>
  );
}
