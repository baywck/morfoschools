"use client";

import { AuthProvider, useAuth } from "@/lib/auth-provider";
import { ThemeProvider } from "@/lib/use-theme";
import { ToastProvider } from "@/components/ui/toast";
import { PromptProvider } from "@/components/ui/prompt-dialog";
import { AppShell } from "@/components/layout/app-shell";
import { QuestionMagicModalHost } from "@/components/ai/question-magic-modal";
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
  LibraryBig,
  Settings,
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

function TenantBrandLogo() {
  const { session } = useAuth();
  const tenant = session?.effectiveTenant;
  const logoUrl = tenant?.logoUrl;

  if (!logoUrl) {
    return <img src="/logo.png" alt="Morfoschools" className="h-7 w-7" />;
  }

  return (
    <span className="flex h-9 w-9 items-center justify-center overflow-hidden p-1 ">
      <img
        src={logoUrl}
        alt={tenant?.name ? `${tenant.name} logo` : "Tenant logo"}
        className="h-full w-full object-contain"
        onError={(event) => {
          event.currentTarget.src = "/logo.png";
          event.currentTarget.className = "h-7 w-7 object-contain";
        }}
      />
    </span>
  );
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
          { label: "Master CP", href: "/app/curriculum-cp", icon: LibraryBig },
          // Stimuli library hidden from sidebar (Opsi B). Stimuli
          // are inline by default — each one bound to its group via
          // exam_question_groups snapshots. Power users can still
          // hit /app/stimuli directly to manage shared library, but
          // 90% of authoring happens inline so we don't promote it.
        ]
      : []),
    { label: "Profile Settings", href: "/app/settings", icon: Settings },
  ];

  return (
    <AuthGuard>
      <AppShell navigation={navigation} brand={<TenantBrandLogo />}>
        {children}
      </AppShell>
      <QuestionMagicModalHost />
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
