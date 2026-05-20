"use client";

import { AuthProvider, useAuth } from "@/lib/auth-provider";
import { ThemeProvider } from "@/lib/use-theme";
import { ToastProvider } from "@/components/ui/toast";
import { PromptProvider } from "@/components/ui/prompt-dialog";
import { AppShell } from "@/components/layout/app-shell";
import { Skeleton } from "@/components/ui/skeleton";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import {
  LayoutDashboard,
  Building2,
  Users,
  GraduationCap,
  BookOpen,
  Briefcase,
  CalendarRange,
  School2,
  FileText,
  ClipboardCheck,
  ClipboardList,
  Library,
} from "lucide-react";
import type { NavItem } from "@/components/layout/sidebar";

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
      <div className="flex h-screen items-center justify-center bg-[var(--shell)]">
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

  const navigation: NavItem[] = [
    { label: "Dashboard", href: "/app", icon: LayoutDashboard },
    ...(session?.roles.includes("master_admin")
      ? [{ label: "Tenants", href: "/app/tenants", icon: Building2 }]
      : []),
    ...(session?.effectiveTenantId
      ? [
          { label: "Academic", href: "/app/academic", icon: CalendarRange },
          { label: "Subjects", href: "/app/subjects", icon: BookOpen },
          { label: "Admin", href: "/app/admin", icon: Users },
          { label: "Teachers", href: "/app/teachers", icon: GraduationCap },
          { label: "Classes", href: "/app/classes", icon: School2 },
          { label: "Students", href: "/app/students", icon: BookOpen },
          { label: "Staff", href: "/app/staff", icon: Briefcase },
          { label: "Programs", href: "/app/programs", icon: BookOpen },
          { label: "Courses", href: "/app/courses", icon: FileText },
          { label: "Exams", href: "/app/exams", icon: ClipboardCheck },
          { label: "Blueprints", href: "/app/blueprints", icon: ClipboardList },
          { label: "Stimuli", href: "/app/stimuli", icon: Library },
        ]
      : []),
  ];

  return (
    <AuthGuard>
      <AppShell navigation={navigation}>
        {children}
      </AppShell>
    </AuthGuard>
  );
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider>
      <AuthProvider>
        <ToastProvider>
          <PromptProvider>
            <AppLayoutInner>{children}</AppLayoutInner>
          </PromptProvider>
        </ToastProvider>
      </AuthProvider>
    </ThemeProvider>
  );
}
