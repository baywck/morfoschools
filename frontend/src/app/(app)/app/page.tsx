"use client";

import { useAuth } from "@/lib/auth-provider";
import {
  Users,
  GraduationCap,
  BookOpen,
  School2,
  CalendarRange,
  TrendingUp,
  ArrowUpRight,
  Clock,
  Sparkles,
  Building2,
  UserPlus,
  FileText,
  Activity,
} from "lucide-react";
import Link from "next/link";
import { cn } from "@/lib/cn";

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return "Selamat pagi";
  if (hour < 17) return "Selamat siang";
  return "Selamat malam";
}

function getFormattedDate(): string {
  return new Date().toLocaleDateString("id-ID", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

/* ─── Stat Card ─── */
interface StatCardProps {
  label: string;
  value: string | number;
  change?: string;
  changePositive?: boolean;
  icon: React.ReactNode;
  gradient: string;
  iconBg: string;
}

function StatCard({ label, value, change, changePositive, icon, gradient, iconBg }: StatCardProps) {
  return (
    <div className="group relative overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--card)] p-5 transition-all duration-300 hover:shadow-[0_8px_30px_rgba(0,0,0,0.06)] hover:border-[var(--border-strong)] hover:-translate-y-0.5">
      {/* Subtle gradient overlay */}
      <div className={cn("absolute inset-0 opacity-[0.03] transition-opacity duration-300 group-hover:opacity-[0.06]", gradient)} />

      <div className="relative flex items-start justify-between">
        <div className="space-y-2">
          <p className="text-[11px] font-semibold uppercase tracking-wider text-[var(--muted-foreground)]">
            {label}
          </p>
          <p className="text-2xl font-bold tracking-tight text-[var(--foreground)]">
            {value}
          </p>
          {change && (
            <div className="flex items-center gap-1">
              <TrendingUp
                size={11}
                className={changePositive ? "text-[var(--success)]" : "text-[var(--muted-foreground)]"}
              />
              <span
                className={cn(
                  "text-[10px] font-medium",
                  changePositive ? "text-[var(--success)]" : "text-[var(--muted-foreground)]"
                )}
              >
                {change}
              </span>
            </div>
          )}
        </div>
        <div className={cn("flex h-10 w-10 items-center justify-center rounded-xl transition-transform duration-300 group-hover:scale-110", iconBg)}>
          {icon}
        </div>
      </div>
    </div>
  );
}

/* ─── Quick Action ─── */
interface QuickActionProps {
  label: string;
  description: string;
  href: string;
  icon: React.ReactNode;
  color: string;
}

function QuickAction({ label, description, href, icon, color }: QuickActionProps) {
  return (
    <Link
      href={href}
      className="group flex items-center gap-3.5 rounded-xl border border-[var(--border)] bg-[var(--card)] p-3.5 transition-all duration-200 hover:border-[var(--border-strong)] hover:shadow-sm hover:bg-[var(--accent)]"
    >
      <div className={cn("flex h-9 w-9 shrink-0 items-center justify-center rounded-lg", color)}>
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-[12px] font-semibold text-[var(--foreground)] group-hover:text-[var(--brand)]  transition-colors">
          {label}
        </p>
        <p className="text-[10px] text-[var(--muted-foreground)] truncate">{description}</p>
      </div>
      <ArrowUpRight
        size={14}
        className="shrink-0 text-[var(--muted-foreground)] opacity-0 -translate-x-1 transition-all duration-200 group-hover:opacity-100 group-hover:translate-x-0"
      />
    </Link>
  );
}

/* ─── Activity Item ─── */
interface ActivityItemProps {
  title: string;
  time: string;
  icon: React.ReactNode;
  iconBg: string;
  isLast?: boolean;
}

function ActivityItem({ title, time, icon, iconBg, isLast }: ActivityItemProps) {
  return (
    <div className="flex gap-3">
      {/* Timeline line + dot */}
      <div className="flex flex-col items-center">
        <div className={cn("flex h-7 w-7 shrink-0 items-center justify-center rounded-full", iconBg)}>
          {icon}
        </div>
        {!isLast && <div className="w-px flex-1 bg-[var(--border)] mt-1.5" />}
      </div>
      {/* Content */}
      <div className={cn("pb-5", isLast && "pb-0")}>
        <p className="text-[12px] font-medium text-[var(--foreground)] leading-snug">{title}</p>
        <p className="text-[10px] text-[var(--muted-foreground)] mt-0.5 flex items-center gap-1">
          <Clock size={9} />
          {time}
        </p>
      </div>
    </div>
  );
}

/* ─── Main Dashboard ─── */
export default function DashboardPage() {
  const { session } = useAuth();

  const isMasterAdmin = session?.roles.includes("master_admin");
  const hasTenant = !!session?.effectiveTenantId;

  // Mock stats — replace with real API calls
  const stats: StatCardProps[] = isMasterAdmin
    ? [
        {
          label: "Total Sekolah",
          value: 12,
          change: "+2 bulan ini",
          changePositive: true,
          icon: <Building2 size={18} className="text-[var(--brand)]" />,
          gradient: "bg-gradient-to-br from-blue-500 to-blue-600",
          iconBg: "bg-[var(--brand-soft)]",
        },
        {
          label: "Total Siswa",
          value: "2,847",
          change: "+124 bulan ini",
          changePositive: true,
          icon: <Users size={18} className="text-[var(--success)]" />,
          gradient: "bg-gradient-to-br from-emerald-500 to-emerald-600",
          iconBg: "bg-[var(--success-soft)]",
        },
        {
          label: "Total Guru",
          value: 186,
          change: "+8 bulan ini",
          changePositive: true,
          icon: <GraduationCap size={18} className="text-[var(--info)]" />,
          gradient: "bg-gradient-to-br from-indigo-500 to-indigo-600",
          iconBg: "bg-[var(--info-soft)]",
        },
        {
          label: "Tahun Ajaran Aktif",
          value: "2025/2026",
          icon: <CalendarRange size={18} className="text-[var(--warning)]" />,
          gradient: "bg-gradient-to-br from-amber-500 to-amber-600",
          iconBg: "bg-[var(--warning-soft)]",
        },
      ]
    : [
        {
          label: "Siswa Aktif",
          value: 342,
          change: "+18 bulan ini",
          changePositive: true,
          icon: <Users size={18} className="text-[var(--brand)]" />,
          gradient: "bg-gradient-to-br from-blue-500 to-blue-600",
          iconBg: "bg-[var(--brand-soft)]",
        },
        {
          label: "Guru & Staff",
          value: 28,
          change: "+2 bulan ini",
          changePositive: true,
          icon: <GraduationCap size={18} className="text-[var(--success)]" />,
          gradient: "bg-gradient-to-br from-emerald-500 to-emerald-600",
          iconBg: "bg-[var(--success-soft)]",
        },
        {
          label: "Kelas",
          value: 14,
          icon: <School2 size={18} className="text-[var(--info)]" />,
          gradient: "bg-gradient-to-br from-indigo-500 to-indigo-600",
          iconBg: "bg-[var(--info-soft)]",
        },
        {
          label: "Mata Pelajaran",
          value: 22,
          icon: <BookOpen size={18} className="text-[var(--warning)]" />,
          gradient: "bg-gradient-to-br from-amber-500 to-amber-600",
          iconBg: "bg-[var(--warning-soft)]",
        },
      ];

  const quickActions: QuickActionProps[] = isMasterAdmin
    ? [
        {
          label: "Tambah Sekolah",
          description: "Daftarkan tenant baru",
          href: "/app/tenants",
          icon: <Building2 size={15} className="text-[var(--brand)]" />,
          color: "bg-[var(--brand-soft)]",
        },
        {
          label: "Kelola Admin",
          description: "Atur admin sekolah",
          href: "/app/admin",
          icon: <UserPlus size={15} className="text-[var(--info)]" />,
          color: "bg-[var(--info-soft)]",
        },
        {
          label: "Laporan",
          description: "Lihat statistik platform",
          href: "/app/courses",
          icon: <FileText size={15} className="text-[var(--success)]" />,
          color: "bg-[var(--success-soft)]",
        },
      ]
    : [
        {
          label: "Tambah Siswa",
          description: "Daftarkan siswa baru",
          href: "/app/students",
          icon: <UserPlus size={15} className="text-[var(--brand)]" />,
          color: "bg-[var(--brand-soft)]",
        },
        {
          label: "Kelola Kelas",
          description: "Atur kelas & seksi",
          href: "/app/classes",
          icon: <School2 size={15} className="text-[var(--info)]" />,
          color: "bg-[var(--info-soft)]",
        },
        {
          label: "Tahun Ajaran",
          description: "Konfigurasi akademik",
          href: "/app/academic",
          icon: <CalendarRange size={15} className="text-[var(--success)]" />,
          color: "bg-[var(--success-soft)]",
        },
      ];

  const activities: ActivityItemProps[] = [
    {
      title: "Siswa baru terdaftar: Ahmad Fauzi",
      time: "5 menit lalu",
      icon: <UserPlus size={12} className="text-[var(--brand)]" />,
      iconBg: "bg-[var(--brand-soft)]",
    },
    {
      title: "Kelas X-IPA 2 diperbarui",
      time: "1 jam lalu",
      icon: <School2 size={12} className="text-[var(--info)]" />,
      iconBg: "bg-[var(--info-soft)]",
    },
    {
      title: "Guru baru ditambahkan: Siti Rahayu",
      time: "3 jam lalu",
      icon: <GraduationCap size={12} className="text-[var(--success)]" />,
      iconBg: "bg-[var(--success-soft)]",
    },
    {
      title: "Tahun ajaran 2025/2026 diaktifkan",
      time: "Kemarin",
      icon: <CalendarRange size={12} className="text-[var(--warning)]" />,
      iconBg: "bg-[var(--warning-soft)]",
    },
    {
      title: "Mata pelajaran Matematika ditambahkan",
      time: "2 hari lalu",
      icon: <BookOpen size={12} className="text-[var(--info)]" />,
      iconBg: "bg-[var(--info-soft)]",
    },
  ];

  return (
    <div className="mx-auto w-full max-w-5xl px-4 py-6 md:px-7 md:py-8 lg:px-8 space-y-7">
      {/* ─── Hero Greeting ─── */}
      <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-[var(--shell)] via-[#1a1f2e] to-[#0f172a] p-6 md:p-8">
        {/* Decorative elements */}
        <div className="absolute top-0 right-0 w-64 h-64 bg-[var(--brand)] opacity-[0.07] rounded-full blur-3xl -translate-y-1/2 translate-x-1/4" />
        <div className="absolute bottom-0 left-1/3 w-48 h-48 bg-[var(--info)] opacity-[0.05] rounded-full blur-3xl translate-y-1/2" />

        <div className="relative flex flex-col md:flex-row md:items-center md:justify-between gap-4">
          <div className="space-y-1.5">
            <div className="flex items-center gap-2">
              <Sparkles size={14} className="text-[var(--brand)] animate-pulse" />
              <span className="text-[10px] font-semibold uppercase tracking-widest text-[var(--brand)]">
                Dashboard
              </span>
            </div>
            <h1 className="text-xl md:text-2xl font-bold text-white tracking-tight">
              {getGreeting()}, {session?.user.displayName?.split(" ")[0]}
            </h1>
            <p className="text-[12px] text-white/60">
              {getFormattedDate()}
            </p>
          </div>

          {/* Role badge */}
          <div className="flex items-center gap-2">
            <div className="flex h-9 items-center gap-2 rounded-full bg-white/[0.08] border border-white/[0.1] px-4 backdrop-blur-sm">
              <Activity size={12} className="text-[var(--brand)]" />
              <span className="text-[11px] font-medium text-white/80 capitalize">
                {session?.roles?.[0]?.replace("_", " ")}
              </span>
            </div>
            {hasTenant && (
              <div className="flex h-9 items-center gap-2 rounded-full bg-white/[0.08] border border-white/[0.1] px-4 backdrop-blur-sm">
                <Building2 size={12} className="text-emerald-400" />
                <span className="text-[11px] font-medium text-white/80">
                  Tenant aktif
                </span>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* ─── Stat Cards ─── */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <StatCard key={stat.label} {...stat} />
        ))}
      </div>

      {/* ─── Bottom Grid: Quick Actions + Activity ─── */}
      <div className="grid gap-5 lg:grid-cols-5">
        {/* Quick Actions */}
        <div className="lg:col-span-2 space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-[13px] font-bold text-[var(--foreground)] tracking-tight">
              Aksi Cepat
            </h2>
            <span className="text-[10px] text-[var(--muted-foreground)]">Pintasan</span>
          </div>
          <div className="space-y-2">
            {quickActions.map((action) => (
              <QuickAction key={action.label} {...action} />
            ))}
          </div>
        </div>

        {/* Activity Feed */}
        <div className="lg:col-span-3 space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-[13px] font-bold text-[var(--foreground)] tracking-tight">
              Aktivitas Terbaru
            </h2>
            <span className="text-[10px] text-[var(--muted-foreground)]">7 hari terakhir</span>
          </div>
          <div className="rounded-2xl border border-[var(--border)] bg-[var(--card)] p-5">
            <div className="space-y-0">
              {activities.map((activity, i) => (
                <ActivityItem
                  key={i}
                  {...activity}
                  isLast={i === activities.length - 1}
                />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
