"use client";

import { useState, useRef, useEffect } from "react";
import { useAuth } from "@/lib/auth-provider";
import { Button } from "@/components/ui/button";
import {
  Search,
  PanelLeftClose,
  PanelLeft,
  Moon,
  Sun,
  ChevronDown,
  User,
  Settings,
  LogOut,
} from "lucide-react";
import { cn } from "@/lib/cn";

interface HeaderProps {
  showPanel: boolean;
  onTogglePanel: () => void;
  theme: "light" | "dark";
  onToggleTheme: () => void;
}

export function Header({ showPanel, onTogglePanel, theme, onToggleTheme }: HeaderProps) {
  const { session, logout } = useAuth();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  return (
    <header className="h-[var(--header-height)] flex items-center border-b border-[var(--border)] bg-[var(--card)]/80 backdrop-blur-sm px-5">
      {/* Left */}
      <div className="flex-1" />

      {/* Right */}
      <div className="flex items-center gap-2.5">
        {/* Search */}
        <div className="hidden lg:flex h-8 items-center rounded-lg border border-[var(--border)] bg-[var(--background)] px-3">
          <Search className="h-3.5 w-3.5 text-[var(--muted-foreground)]" />
          <input
            type="text"
            className="bg-transparent text-xs outline-none w-36 ml-2 placeholder:text-[var(--muted-foreground)]"
            placeholder="Search..."
          />
        </div>

        {/* Panel toggle */}
        <Button variant="ghost" size="icon" onClick={onTogglePanel}>
          {showPanel ? <PanelLeftClose className="h-3.5 w-3.5" /> : <PanelLeft className="h-3.5 w-3.5" />}
        </Button>

        {/* Theme toggle */}
        <Button variant="ghost" size="icon" onClick={onToggleTheme}>
          {theme === "light" ? <Moon className="h-3.5 w-3.5" /> : <Sun className="h-3.5 w-3.5" />}
        </Button>

        {/* Divider */}
        <div className="h-6 w-px bg-[var(--border)]" />

        {/* User dropdown */}
        <div className="relative" ref={dropdownRef}>
          <button
            onClick={() => setDropdownOpen((v) => !v)}
            className="flex items-center gap-2.5 rounded-lg px-2 py-1.5 hover:bg-[var(--muted)] -my-1 transition-colors"
          >
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--primary)] text-[10px] font-bold text-[var(--primary-foreground)]">
              {session?.user.displayName?.charAt(0) || "?"}
            </div>
            <div className="hidden sm:block text-left">
              <p className="text-xs font-medium text-[var(--foreground)]">
                {session?.user.displayName}
              </p>
              <p className="text-[10px] text-[var(--muted-foreground)]">
                {session?.roles?.[0]?.replace("_", " ") || "User"}
              </p>
            </div>
            <ChevronDown
              className={cn(
                "h-3 w-3 text-[var(--muted-foreground)] transition-transform",
                dropdownOpen && "rotate-180"
              )}
            />
          </button>

          {/* Dropdown */}
          {dropdownOpen && (
            <div className="absolute right-0 top-full mt-2 w-52 rounded-lg border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg z-50">
              {/* Profile section */}
              <div className="px-3 py-2.5 border-b border-[var(--border)]">
                <p className="text-xs font-medium text-[var(--foreground)]">
                  {session?.user.displayName}
                </p>
                <p className="text-[10px] text-[var(--muted-foreground)]">
                  {session?.user.email}
                </p>
              </div>

              {/* Links */}
              <div className="py-1">
                <button className="flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-xs text-[var(--foreground)] hover:bg-[var(--muted)] transition-colors">
                  <User className="h-3.5 w-3.5" />
                  Profile
                </button>
                <button className="flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-xs text-[var(--foreground)] hover:bg-[var(--muted)] transition-colors">
                  <Settings className="h-3.5 w-3.5" />
                  Settings
                </button>
              </div>

              {/* Logout */}
              <div className="border-t border-[var(--border)] pt-1">
                <button
                  onClick={() => { logout(); setDropdownOpen(false); }}
                  className="flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-xs text-[var(--danger)] hover:bg-[var(--danger-soft)] transition-colors"
                >
                  <LogOut className="h-3.5 w-3.5" />
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
