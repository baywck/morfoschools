// Morfosis — MobileNav (bottom nav in dark shell, horizontal scroll)
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";
import { cn } from "@/lib/cn";
import type { NavItem } from "@/components/layout/sidebar";

interface MobileNavProps {
  navigation: NavItem[];
}

export function MobileNav({ navigation }: MobileNavProps) {
  const pathname = usePathname();
  const navRef = useRef<HTMLElement | null>(null);
  const activeRef = useRef<HTMLAnchorElement | null>(null);

  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: "nearest", inline: "center" });
  }, [pathname]);

  return (
    <nav ref={navRef} className="fixed bottom-0 inset-x-0 z-50 flex md:hidden h-16 items-center bg-[var(--shell)] px-2 pb-1 overflow-x-auto scrollbar-none">
      <div className="flex min-w-max items-center gap-3 px-2">
        {navigation.map((item) => {
          const Icon = item.icon;
          const isActive =
            item.href === "/app"
              ? pathname === item.href
              : pathname.startsWith(item.href);

          return (
            <Link
              key={item.href}
              ref={isActive ? activeRef : undefined}
              href={item.href}
              className={cn(
                "flex flex-col items-center gap-0.5 px-3 py-1.5 rounded-lg transition-colors shrink-0",
                isActive
                  ? "text-[var(--shell-foreground)]"
                  : "text-[var(--shell-muted)]"
              )}
            >
              <Icon className="h-5 w-5" />
              <span className="text-[10px] font-medium whitespace-nowrap">{item.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
