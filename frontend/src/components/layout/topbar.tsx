// Morfosis — Topbar (dark shell: breadcrumb + theme + user)
"use client";

import { useState, useRef, useEffect } from "react";
import { usePathname } from "next/navigation";
import { Moon, Sun, LogOut, ChevronDown, Home as HomeIcon } from "lucide-react";
import { useTheme } from "@/lib/use-theme";
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

export function Topbar() {
  const { dark, toggle } = useTheme();
  const { session, logout } = useAuth();
  const pathname = usePathname();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const segments = pathname.split("/").filter(Boolean);
  const displaySegments = segments.filter((s) => s !== "app");
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
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      {/* Right — actions */}
      <div className="flex items-center gap-2">
        {/* Theme toggle — desktop only */}
        <button
          onClick={toggle}
          className="hidden md:flex h-8 w-8 items-center justify-center rounded-lg text-[var(--shell-muted)] hover:text-[var(--shell-foreground)] transition-colors"
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
            {/* Name + sub — desktop only */}
            <div className="hidden md:block text-left">
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
                "hidden md:block text-[var(--shell-muted)] transition-transform",
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
