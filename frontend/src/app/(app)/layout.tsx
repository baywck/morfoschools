"use client";

import { AuthProvider, useAuth } from "@/lib/auth-provider";
import { ToastProvider } from "@/components/ui/toast";
import { AppShell } from "@/components/app-shell";
import type { NavSection, NavItem } from "@/components/app-shell/types";
import {
  LayoutDashboard,
  Building2,
  Users,
  CalendarRange,
  School2,
  BookOpen,
  GraduationCap,
  Settings,
  Bell,
} from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

function AuthGuard({ children }: { children: React.ReactNode }) {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) {
      router.replace("/login");
    }
  }, [loading, session, router]);

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-[var(--background)]">
        <div className="flex flex-col items-center gap-3">
          <Skeleton className="h-10 w-10 rounded-full" />
          <Skeleton className="h-4 w-32" />
        </div>
      </div>
    );
  }

  if (!session) return null;
  return <>{children}</>;
}

function AppLayoutInner({ children }: { children: React.ReactNode }) {
  const { session } = useAuth();

  const sections: NavSection[] = [
    {
      title: "Main",
      items: [
        { label: "Dashboard", href: "/app", icon: LayoutDashboard },
        ...(session?.roles.includes("master_admin")
          ? [{ label: "Tenants", href: "/app/tenants", icon: Building2 }]
          : []),
        ...(session?.effectiveTenantId
          ? [
              { label: "Users", href: "/app/users", icon: Users },
              { label: "Academic", href: "/app/academic", icon: CalendarRange },
              { label: "Classes", href: "/app/classes", icon: School2 },
            ]
          : []),
      ],
    },
    ...(session?.effectiveTenantId
      ? [
          {
            title: "Learning",
            items: [
              { label: "Programs", href: "/app/programs", icon: BookOpen },
              { label: "Exams", href: "/app/exams", icon: GraduationCap },
            ] as NavItem[],
          },
        ]
      : []),
  ];

  const bottomItems: NavItem[] = [
    { label: "Notifications", href: "/app/notifications", icon: Bell },
    { label: "Settings", href: "/app/settings", icon: Settings },
  ];

  return (
    <AuthGuard>
      <AppShell sections={sections} bottomItems={bottomItems}>
        {children}
      </AppShell>
    </AuthGuard>
  );
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthProvider>
      <ToastProvider>
        <AppLayoutInner>{children}</AppLayoutInner>
      </ToastProvider>
    </AuthProvider>
  );
}
