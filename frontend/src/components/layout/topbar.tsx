// Morfosis — Topbar (shell-aware: breadcrumb + theme + user)
"use client";

import { useState, useRef, useEffect } from "react";
import { usePathname } from "next/navigation";
import { Moon, Sun, LogOut, ChevronDown, Home as HomeIcon, Bot, Palette, Monitor, SunMoon } from "lucide-react";
import { useTheme, type ShellMode, type AccentColor } from "@/lib/use-theme";
import { useAuth } from "@/lib/auth-provider";
import { cn } from "@/lib/cn";
import {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";

function formatSegment(segment: string) {
  return segment
    .split("-")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}

function truncateMobileBreadcrumb(label: string) {
  return label.length > 12 ? `${label.slice(0, 12)}...` : label;
}

interface TopbarProps {
  onToggleAiChat?: () => void;
  aiChatOpen?: boolean;
}

const accentOptions: { value: AccentColor; label: string; color: string }[] = [
  { value: "blue", label: "Blue", color: "bg-blue-500" },
  { value: "violet", label: "Violet", color: "bg-violet-500" },
  { value: "emerald", label: "Emerald", color: "bg-emerald-500" },
  { value: "rose", label: "Rose", color: "bg-rose-500" },
  { value: "amber", label: "Amber", color: "bg-amber-500" },
  { value: "indigo", label: "Indigo", color: "bg-indigo-500" },
];

export function Topbar({ onToggleAiChat, aiChatOpen }: TopbarProps) {
  const { dark, toggle, shellMode, setShellMode, accent, setAccent } = useTheme();
  const { session, logout } = useAuth();
  const pathname = usePathname();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [themeMenuOpen, setThemeMenuOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const themeMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
      }
      if (themeMenuRef.current && !themeMenuRef.current.contains(e.target as Node)) {
        setThemeMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const segments = pathname.split("/").filter(Boolean);
  const displaySegments = segments.filter((s) => s !== "app");
  const mobileDisplaySegments = displaySegments.length > 1 ? ["..", displaySegments[displaySegments.length - 1]] : displaySegments;
  const roleLabel = session?.roles?.[0]?.replace("_", " ") || "User";

  return (
    <header className="flex h-[var(--header-height)] items-center gap-3 px-4">
      {/* Left — Breadcrumb */}
      <div className="flex-1 min-w-0">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink href="/app">
                <HomeIcon size={14} />
                <span className="sr-only">Home</span>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <span className="contents md:hidden">
              {mobileDisplaySegments.map((segment, index) => {
                const isEllipsis = segment === "..";
                const isLast = index === mobileDisplaySegments.length - 1;
                const label = isEllipsis ? ".." : truncateMobileBreadcrumb(formatSegment(segment));
                return (
                  <span key={`${segment}-${index}`} className="contents">
                    <BreadcrumbSeparator />
                    <BreadcrumbItem>
                      {isLast ? (
                        <BreadcrumbPage title={isEllipsis ? undefined : formatSegment(segment)}>{label}</BreadcrumbPage>
                      ) : (
                        <span className="text-[var(--muted-foreground)]">{label}</span>
                      )}
                    </BreadcrumbItem>
                  </span>
                );
              })}
            </span>
            <span className="contents max-md:hidden">
              {displaySegments.map((segment, index) => (
                <span key={segment} className="contents">
                  <BreadcrumbSeparator />
                  <BreadcrumbItem>
                    {index === displaySegments.length - 1 ? (
                      <BreadcrumbPage>{formatSegment(segment)}</BreadcrumbPage>
                    ) : (
                      <BreadcrumbLink href={`/app/${displaySegments.slice(0, index + 1).join("/")}`}>
                        {formatSegment(segment)}
                      </BreadcrumbLink>
                    )}
                  </BreadcrumbItem>
                </span>
              ))}
            </span>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      {/* Right — actions */}
      <div className="flex items-center gap-2">
        {/* AI Chat toggle */}
        {onToggleAiChat && (
          <button
            onClick={onToggleAiChat}
            className={cn(
              "flex h-8 w-8 items-center justify-center rounded-lg transition-colors",
              aiChatOpen ? "bg-[var(--muted)] text-[var(--foreground)]" : "text-[var(--muted-foreground)] hover:text-[var(--foreground)] hover:bg-[var(--muted)]"
            )}
            aria-label="Toggle AI Chat"
          >
            <Bot size={16} strokeWidth={2} />
          </button>
        )}

        {/* Theme settings — desktop only */}
        <div className="relative hidden md:block" ref={themeMenuRef}>
          <button
            onClick={() => setThemeMenuOpen((v) => !v)}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--muted-foreground)] hover:text-[var(--foreground)] hover:bg-[var(--muted)] transition-colors"
            aria-label="Theme settings"
          >
            <Palette size={16} strokeWidth={2} />
          </button>

          {themeMenuOpen && (
            <div className="absolute right-0 top-full mt-2 w-56 rounded-xl border border-[var(--border)] bg-[var(--card)] p-3 shadow-xl z-50 space-y-3">
              {/* Color mode */}
              <div>
                <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--muted-foreground)] mb-2">Color Mode</p>
                <div className="flex gap-1">
                  <button
                    onClick={() => { if (dark) toggle(); }}
                    className={cn(
                      "flex-1 flex items-center justify-center gap-1.5 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors",
                      !dark ? "bg-[var(--primary)] text-[var(--primary-foreground)]" : "text-[var(--muted-foreground)] hover:bg-[var(--muted)]"
                    )}
                  >
                    <Sun size={12} /> Light
                  </button>
                  <button
                    onClick={() => { if (!dark) toggle(); }}
                    className={cn(
                      "flex-1 flex items-center justify-center gap-1.5 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors",
                      dark ? "bg-[var(--primary)] text-[var(--primary-foreground)]" : "text-[var(--muted-foreground)] hover:bg-[var(--muted)]"
                    )}
                  >
                    <Moon size={12} /> Dark
                  </button>
                </div>
              </div>

              {/* Shell mode */}
              <div>
                <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--muted-foreground)] mb-2">Shell Style</p>
                <div className="flex gap-1">
                  <button
                    onClick={() => setShellMode("dark")}
                    className={cn(
                      "flex-1 flex items-center justify-center gap-1.5 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors",
                      shellMode === "dark" ? "bg-[var(--primary)] text-[var(--primary-foreground)]" : "text-[var(--muted-foreground)] hover:bg-[var(--muted)]"
                    )}
                  >
                    <Monitor size={12} /> Dark
                  </button>
                  <button
                    onClick={() => setShellMode("light")}
                    className={cn(
                      "flex-1 flex items-center justify-center gap-1.5 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors",
                      shellMode === "light" ? "bg-[var(--primary)] text-[var(--primary-foreground)]" : "text-[var(--muted-foreground)] hover:bg-[var(--muted)]"
                    )}
                  >
                    <SunMoon size={12} /> Light
                  </button>
                </div>
              </div>

              {/* Accent color */}
              <div>
                <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--muted-foreground)] mb-2">Accent Color</p>
                <div className="grid grid-cols-6 gap-1.5">
                  {accentOptions.map((opt) => (
                    <button
                      key={opt.value}
                      onClick={() => setAccent(opt.value)}
                      title={opt.label}
                      className={cn(
                        "h-7 w-7 rounded-full transition-all",
                        opt.color,
                        accent === opt.value
                          ? "ring-2 ring-[var(--foreground)] ring-offset-2 ring-offset-[var(--card)] scale-110"
                          : "hover:scale-110 opacity-70 hover:opacity-100"
                      )}
                    />
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        {/* User button */}
        <div className="relative" ref={dropdownRef}>
          <button
            onClick={() => setDropdownOpen((v) => !v)}
            className="flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-[var(--muted)] transition-colors"
          >
            <div className="flex h-7 w-7 items-center justify-center rounded-full bg-[var(--brand-soft)] text-[10px] font-bold text-[var(--brand-strong)]">
              {session?.user.displayName?.charAt(0) || "?"}
            </div>
            {/* Name + sub — desktop only */}
            <div className="hidden md:block text-left">
              <p className="text-[12px] font-medium text-[var(--foreground)] leading-tight">
                {session?.user.displayName}
              </p>
              <p className="text-[10px] text-[var(--muted-foreground)] leading-tight capitalize">
                {roleLabel}
              </p>
            </div>
            <ChevronDown
              size={12}
              className={cn(
                "hidden md:block text-[var(--muted-foreground)] transition-transform",
                dropdownOpen && "rotate-180"
              )}
            />
          </button>

          {/* Dropdown */}
          {dropdownOpen && (
            <div className="absolute right-0 top-full mt-2 w-48 rounded-xl border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg z-50">
              <div className="px-3 py-2.5 border-b border-[var(--border)]">
                <p className="text-[12px] font-medium text-[var(--foreground)]">
                  {session?.user.displayName}
                </p>
                <p className="text-[10px] text-[var(--muted-foreground)] mt-0.5">
                  {session?.user.email}
                </p>
              </div>
              {/* Theme toggle — mobile (inside dropdown) */}
              <button
                onClick={toggle}
                className="flex md:hidden w-full items-center gap-2.5 rounded-lg px-3 py-2 text-[11px] font-medium text-[var(--foreground)] hover:bg-[var(--muted)] transition-colors"
              >
                {dark ? <Sun size={13} /> : <Moon size={13} />}
                {dark ? "Light mode" : "Dark mode"}
              </button>
              <button
                onClick={() => setShellMode(shellMode === "dark" ? "light" : "dark")}
                className="flex md:hidden w-full items-center gap-2.5 rounded-lg px-3 py-2 text-[11px] font-medium text-[var(--foreground)] hover:bg-[var(--muted)] transition-colors"
              >
                <SunMoon size={13} />
                {shellMode === "dark" ? "Light shell" : "Dark shell"}
              </button>
              <div className="border-t border-[var(--border)] pt-1 mt-1">
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
