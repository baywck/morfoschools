"use client";

/**
 * StimulusLibraryModal — full-width centered modal for browsing the
 * stimulus library when picking a passage to attach to a group or
 * question. Replaces the cramped inline picker for the library tab
 * since stimuli are typically long passages that need real reading
 * room.
 *
 * Layout: search bar + grid of cards. Each card shows title +
 * lifecycle badge + body preview (~3 lines). Click a card to confirm
 * selection.
 */

import { useEffect, useState } from "react";
import { Loader2, Search, X, FileText } from "lucide-react";
import { listStimuli, type Stimulus } from "@/lib/modules-api";
import { stripHtmlPreview } from "@/components/ui/rendered-content";
import { cn } from "@/lib/cn";

interface StimulusLibraryModalProps {
  open: boolean;
  onClose: () => void;
  onSelect: (s: Stimulus) => void;
}

export function StimulusLibraryModal({ open, onClose, onSelect }: StimulusLibraryModalProps) {
  const [search, setSearch] = useState("");
  const [items, setItems] = useState<Stimulus[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    async function load() {
      setLoading(true);
      const res = await listStimuli({ search: search.trim() || undefined, lifecycle: "shared" });
      if (cancelled) return;
      if (res.data) setItems(res.data.data);
      setLoading(false);
    }
    const t = setTimeout(load, search ? 220 : 0);
    return () => { cancelled = true; clearTimeout(t); };
  }, [open, search]);

  // Close on Escape.
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4 md:p-8" onClick={onClose}>
      <div
        className="flex h-[80vh] w-full max-w-3xl flex-col overflow-hidden rounded-2xl bg-[var(--card)] shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center gap-3 border-b border-[var(--border)] px-5 py-3">
          <FileText className="h-4 w-4 text-[var(--brand)] shrink-0" />
          <h3 className="flex-1 text-[14px] font-semibold text-[var(--foreground)]">
            Library Stimulus
          </h3>
          <button
            type="button"
            onClick={onClose}
            className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)] transition-colors"
            aria-label="Tutup"
          >
            <X size={14} />
          </button>
        </div>

        {/* Search */}
        <div className="border-b border-[var(--border)] px-5 py-3">
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--muted-foreground)]" size={14} />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Cari judul stimulus…"
              className="h-9 w-full rounded-md border border-[var(--border)] bg-[var(--background)] pl-9 pr-3 text-[13px] text-[var(--foreground)] outline-none focus:border-[var(--brand)] focus:ring-2 focus:ring-[var(--field-ring)]"
              autoFocus
            />
          </div>
        </div>

        {/* List */}
        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-3">
          {loading && (
            <div className="flex items-center justify-center py-12 text-[var(--muted-foreground)]">
              <Loader2 className="h-5 w-5 animate-spin" />
            </div>
          )}
          {!loading && items.length === 0 && (
            <div className="py-12 text-center text-[13px] text-[var(--muted-foreground)]">
              {search.trim() ? "Tidak ada stimulus yang cocok." : "Library masih kosong. Tulis stimulus baru saja."}
            </div>
          )}
          {!loading && items.map((s) => (
            <button
              key={s.id}
              type="button"
              onClick={() => { onSelect(s); onClose(); }}
              className={cn(
                "block w-full rounded-xl border border-[var(--border)] bg-[var(--background)] p-4 text-left transition-colors",
                "hover:border-[var(--brand)] hover:bg-[var(--brand-soft)]/30",
              )}
            >
              <div className="mb-1.5 flex items-center gap-2">
                <h4 className="flex-1 text-[13px] font-semibold text-[var(--foreground)] line-clamp-1">
                  {s.title}
                </h4>
                <span className="inline-flex items-center rounded-full bg-[var(--muted)] px-2 py-0.5 text-[9.5px] font-bold uppercase tracking-wider text-[var(--muted-foreground)]">
                  {s.lifecycle === "shared" ? "Shared" : s.lifecycle === "exam_scoped" ? "Local" : s.lifecycle}
                </span>
              </div>
              <p className="text-[12px] leading-relaxed text-[var(--muted-foreground)] line-clamp-3">
                {stripHtmlPreview(s.content, 280)}
              </p>
              {s.source && (
                <p className="mt-1.5 text-[10.5px] italic text-[var(--muted-foreground)]">
                  Sumber: {s.source}
                </p>
              )}
            </button>
          ))}
        </div>

        {/* Footer */}
        <div className="border-t border-[var(--border)] px-5 py-2.5 text-[10.5px] text-[var(--muted-foreground)]">
          Klik kartu untuk import title + body ke editor. Konten akan dapat diedit setelah diimpor.
        </div>
      </div>
    </div>
  );
}
