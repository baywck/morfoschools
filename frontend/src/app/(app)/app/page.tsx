"use client";

import { useAuth } from "@/lib/auth-provider";
import { LayoutDashboard } from "lucide-react";

export default function DashboardPage() {
  const { session } = useAuth();

  return (
    <div>
      {/* Page Header — NOT a card */}
      <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-lg font-semibold text-[var(--foreground)]">Dashboard</h1>
          <p className="mt-0.5 text-sm text-[var(--muted-foreground)]">
            Selamat datang, {session?.user.displayName}
          </p>
        </div>
      </div>

      {/* Metric Cards */}
      <div className="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {session?.roles.map((role) => (
          <div
            key={role}
            className="rounded-lg border border-[var(--border)] bg-[var(--card)] p-4"
          >
            <div className="flex items-center gap-2">
              <LayoutDashboard className="h-3.5 w-3.5 text-[var(--muted-foreground)]" />
              <p className="text-xs font-medium text-[var(--muted-foreground)]">Role</p>
            </div>
            <p className="mt-2 text-2xl font-semibold text-[var(--foreground)] capitalize">
              {role.replace("_", " ")}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}
