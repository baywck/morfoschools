"use client";

import { useAuth } from "@/lib/auth-provider";

export default function DashboardPage() {
  const { session } = useAuth();

  return (
    <div className="space-y-6">
      {/* Welcome */}
      <div>
        <h2 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight">
          Selamat datang, {session?.user.displayName}
        </h2>
        <p className="text-[12px] text-[var(--muted-foreground)] mt-0.5">
          {session?.roles?.[0]?.replace("_", " ")} • {session?.effectiveTenantId ? "Tenant active" : "Platform level"}
        </p>
      </div>

      {/* Metric Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {session?.roles.map((role) => (
          <div
            key={role}
            className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4"
          >
            <p className="text-[11px] font-medium text-[var(--muted-foreground)] uppercase tracking-wider">Role</p>
            <p className="mt-2 text-xl font-semibold text-[var(--foreground)] capitalize">
              {role.replace("_", " ")}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}
