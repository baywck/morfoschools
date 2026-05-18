"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/cn";
import type { NavSection } from "./types";

interface MobileNavProps {
  sections: NavSection[];
}

export function MobileNav({ sections }: MobileNavProps) {
  const pathname = usePathname();
  const items = sections.flatMap((s) => s.items).slice(0, 5);

  return (
    <nav className="fixed bottom-0 inset-x-0 z-50 flex lg:hidden h-14 items-center justify-around border-t border-[var(--border)] bg-[var(--card)]">
      {items.map((item) => {
        const Icon = item.icon;
        const isActive = pathname === item.href || pathname.startsWith(item.href + "/");
        return (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              "flex flex-col items-center gap-0.5 px-3 py-1 rounded-lg",
              isActive ? "text-[var(--foreground)]" : "text-[var(--muted-foreground)]"
            )}
          >
            <Icon className="h-5 w-5" />
            <span className="text-[10px] font-medium">{item.label}</span>
          </Link>
        );
      })}
    </nav>
  );
}
