"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-provider";
import {
  LayoutDashboard,
  Building2,
  Users,
  CalendarRange,
  School2,
  BookOpen,
  GraduationCap,
  LogOut,
} from "lucide-react";
import { cn } from "@/lib/cn";

interface NavItem {
  label: string;
  href: string;
  icon: React.ElementType;
  roles?: string[];
  requiresTenant?: boolean;
}

const navigation: NavItem[] = [
  { label: "Dashboard", href: "/app", icon: LayoutDashboard },
  { label: "Tenants", href: "/app/tenants", icon: Building2, roles: ["master_admin"] },
  { label: "Users", href: "/app/users", icon: Users, requiresTenant: true },
  { label: "Academic", href: "/app/academic", icon: CalendarRange, requiresTenant: true },
  { label: "Classes", href: "/app/classes", icon: School2, requiresTenant: true },
  { label: "Programs", href: "/app/programs", icon: BookOpen, requiresTenant: true },
  { label: "Exams", href: "/app/exams", icon: GraduationCap, requiresTenant: true },
];

export function Sidebar() {
  const pathname = usePathname();
  const { session, logout } = useAuth();

  const visibleItems = navigation.filter((item) => {
    if (item.requiresTenant && !session?.effectiveTenantId) return false;
    if (item.roles && !item.roles.some((r) => session?.roles.includes(r))) return false;
    return true;
  });

  return (
    <aside className="fixed left-0 top-0 z-40 flex h-screen w-[var(--sidebar-width)] flex-col items-center bg-[color:var(--sidebar-bg)] py-4">
      {/* Logo */}
      <div className="mb-6 flex h-10 w-10 items-center justify-center rounded-xl bg-[color:var(--sidebar-active)]">
        <span className="text-sm font-bold text-white">M</span>
      </div>

      {/* Nav items */}
      <nav className="flex flex-1 flex-col items-center gap-1">
        {visibleItems.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href || pathname.startsWith(item.href + "/");
          return (
            <Link
              key={item.href}
              href={item.href}
              title={item.label}
              className={cn(
                "flex h-10 w-10 items-center justify-center rounded-xl transition-all",
                isActive
                  ? "bg-[color:var(--sidebar-active)] text-white shadow-sm"
                  : "text-[color:var(--sidebar-fg)] hover:bg-white/10"
              )}
            >
              <Icon className="h-5 w-5" />
            </Link>
          );
        })}
      </nav>

      {/* Logout */}
      <button
        onClick={logout}
        title="Logout"
        className="flex h-10 w-10 items-center justify-center rounded-xl text-[color:var(--sidebar-fg)] transition-all hover:bg-white/10"
      >
        <LogOut className="h-5 w-5" />
      </button>
    </aside>
  );
}
