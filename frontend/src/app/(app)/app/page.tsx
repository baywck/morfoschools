"use client";

import { useAuth } from "@/lib/auth-provider";

export default function DashboardPage() {
  const { session } = useAuth();

  return (
    <div>
      <h1 className="text-2xl font-bold text-[color:var(--foreground)]">Dashboard</h1>
      <p className="mt-2 text-sm text-[color:var(--foreground-muted)]">
        Selamat datang, {session?.user.displayName}
      </p>

      <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {session?.roles.map((role) => (
          <div
            key={role}
            className="rounded-xl border border-[color:var(--border)] bg-[color:var(--surface)] p-4"
          >
            <p className="text-xs text-[color:var(--foreground-muted)]">Role</p>
            <p className="mt-1 text-sm font-medium text-[color:var(--foreground)] capitalize">
              {role.replace("_", " ")}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}
