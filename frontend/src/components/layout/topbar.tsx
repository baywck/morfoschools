// Morfosis — Topbar (dark shell: logo + user button)
"use client";

import { useState, useRef, useEffect } from "react";
import { usePathname } from "next/navigation";
import { Moon, Sun, LogOut, ChevronDown } from "lucide-react";
import { useTheme } from "@/lib/use-theme";
import { useAuth } from "@/lib/auth-provider";
import { cn } from "@/lib/cn";

export function Topbar() {
  const { dark, toggle } = useTheme();
  const { session, logout } = useAuth();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const pathname = usePathname();

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const roleLabel = session?.roles?.[0]?.replace("_", " ") || "User";

  const getPageTitle = () => {
    if (!pathname) return "";
    const parts = pathname.split("/").filter(Boolean);
    const lastPart = parts[parts.length - 1];
    if (!lastPart || lastPart === "app") return "";
    return " / " + lastPart.charAt(0).toUpperCase() + lastPart.slice(1);
  };

  return (
    <header className="flex h-[var(--header-height)] items-center gap-3 px-4 pl-1">
      {/* Left — Logo */}
      <div className="flex items-center gap-2.5 min-w-0 flex-1">
        <img src="/logo.png" alt="Morfoschools" className="h-6 w-6 md:hidden" />
        <span className="text-[13px] font-semibold text-[var(--shell-foreground)] hidden md:inline">
          Morfoschools{getPageTitle()}
        </span>
      </div>

      {/* Right — actions */}
      <div className="flex items-center gap-2">
        {/* Theme toggle — no border/box */}
        <button
          onClick={toggle}
          className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--shell-muted)] hover:text-[var(--shell-foreground)] transition-colors"
          aria-label={dark ? "Switch to light mode" : "Switch to dark mode"}
        >
          {dark ? <Sun size={16} strokeWidth={2} /> : <Moon size={16} strokeWidth={2} />}
        </button>

        {/* User button */}
        <div className="relative" ref={dropdownRef}>
          <button
            onClick={() => setDropdownOpen((v) => !v)}
            className="flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-white/[0.06] transition-colors"
          >
            <div className="flex h-7 w-7 items-center justify-center rounded-full bg-white/15 text-[10px] font-bold text-[var(--shell-foreground)]">
              {session?.user.displayName?.charAt(0) || "?"}
            </div>
            <div className="text-left">
              <p className="text-[12px] font-medium text-[var(--shell-foreground)] leading-tight">
                {session?.user.displayName}
              </p>
              <p className="text-[10px] text-[var(--shell-muted)] leading-tight capitalize">
                {roleLabel}
              </p>
            </div>
            <ChevronDown
              size={12}
              className={cn(
                "text-[var(--shell-muted)] transition-transform",
                dropdownOpen && "rotate-180"
              )}
            />
          </button>

          {/* Dropdown */}
          {dropdownOpen && (
            <div className="absolute right-0 top-full mt-2 w-48 rounded-xl border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg z-50">
              {/* User info */}
              <div className="px-3 py-2.5 border-b border-[var(--border)]">
                <p className="text-[12px] font-medium text-[var(--foreground)]">
                  {session?.user.displayName}
                </p>
                <p className="text-[10px] text-[var(--muted-foreground)] mt-0.5">
                  {session?.user.email}
                </p>
              </div>

              {/* Logout */}
              <div className="pt-1">
                <button
                  onClick={() => { logout(); setDropdownOpen(false); }}
                  className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-[11px] font-medium text-[var(--danger)] hover:bg-[var(--danger-soft)] transition-colors"
                >
                  <LogOut size={13} />
                  Log out
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
