"use client";

import { useAuth } from "@/lib/auth-provider";

export function Header() {
  const { session } = useAuth();

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-[color:var(--border)] bg-[color:var(--surface)] px-6">
      <div className="flex items-center gap-3">
        <h2 className="text-sm font-medium text-[color:var(--foreground)]">
          {session?.effectiveTenantId ? "School Dashboard" : "Platform"}
        </h2>
      </div>
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[color:var(--brand-subtle)]">
            <span className="text-xs font-semibold text-[color:var(--brand)]">
              {session?.user.displayName?.charAt(0) || "?"}
            </span>
          </div>
          <span className="text-sm text-[color:var(--foreground-muted)]">
            {session?.user.displayName}
          </span>
        </div>
      </div>
    </header>
  );
}
