// Morfosis — MobileNav (bottom nav in dark shell)
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/cn";
import type { NavItem } from "@/components/layout/sidebar";

interface MobileNavProps {
  navigation: NavItem[];
}

export function MobileNav({ navigation }: MobileNavProps) {
  const pathname = usePathname();
  const items = navigation.slice(0, 5);

  return (
    <nav className="fixed bottom-0 inset-x-0 z-50 flex md:hidden h-18 items-center justify-around bg-[var(--shell)] px-2 pb-2">
      {items.map((item) => {
        const Icon = item.icon;
        const isActive =
          item.href === "/app"
            ? pathname === item.href
            : pathname.startsWith(item.href);

        return (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              "flex flex-col items-center gap-0.5 px-3 py-1.5 rounded-lg transition-colors",
              isActive
                ? "text-[var(--shell-foreground)]"
                : "text-[var(--shell-muted)]"
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
