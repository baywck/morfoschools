// Morfosis — AppShell (dark shell + floating content card + AI chat push)
"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { Sidebar, type NavItem } from "@/components/layout/sidebar";
import { Topbar } from "@/components/layout/topbar";
import { MobileNav } from "@/components/layout/mobile-nav";
import { AiChatPanel } from "@/components/layout/ai-chat-panel";
import { cn } from "@/lib/cn";

type AppShellProps = {
  children: ReactNode;
  navigation: NavItem[];
  brand?: ReactNode;
};

export function AppShell({ children, navigation, brand }: AppShellProps) {
  const [aiChatOpen, setAiChatOpen] = useState(false);
  const router = useRouter();

  // Listen for inline-magic 'open AI panel' requests from anywhere in
  // the app. Inline-magic actions on cards dispatch this event so the
  // sidebar opens (if closed) and the user sees the AI proposal land.
  useEffect(() => {
    function openPanel() { setAiChatOpen(true); }
    window.addEventListener("morfoschools:open-ai-panel", openPanel);
    return () => window.removeEventListener("morfoschools:open-ai-panel", openPanel);
  }, []);

  // Global listener: when any mutation happens (AI or manual), refresh server data.
  // Pages that fetch data client-side also listen to this event independently.
  useEffect(() => {
    function handleDataChanged() {
      router.refresh();
    }
    window.addEventListener("morfoschools:data-changed", handleDataChanged);
    return () => window.removeEventListener("morfoschools:data-changed", handleDataChanged);
  }, [router]);

  return (
    <div className="h-screen overflow-hidden bg-[var(--shell)]">
      {/* Desktop sidebar */}
      <Sidebar navigation={navigation} brand={brand} />

      {/* Main area — shrinks when AI chat is open */}
      <div className={cn(
        "relative flex h-screen flex-col md:pl-[var(--sidebar-width)] transition-all duration-300",
        aiChatOpen && "md:pr-[360px]"
      )}>
        {/* Topbar — in dark shell */}
        <Topbar onToggleAiChat={() => setAiChatOpen((v) => !v)} aiChatOpen={aiChatOpen} />

        {/* Content — floating white card */}
        <div className="flex-1 min-h-0 px-2 pb-[4.5rem] md:p-0 md:pb-2.5 md:pr-2.5">
          <div className="relative flex h-full flex-col bg-[var(--background)] rounded-2xl shadow-[0_-2px_20px_rgba(0,0,0,0.08)] md:rounded-xl md:border md:border-[var(--border)] md:shadow-[0_20px_60px_rgba(0,0,0,0.08)]">
            <main className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden rounded-[inherit]">
              {children}
            </main>
          </div>
        </div>
      </div>

      {/* Mobile bottom nav */}
      <MobileNav navigation={navigation} />

      {/* AI Chat Panel — fixed right, pushes content via padding */}
      <AiChatPanel open={aiChatOpen} onClose={() => setAiChatOpen(false)} />
    </div>
  );
}
