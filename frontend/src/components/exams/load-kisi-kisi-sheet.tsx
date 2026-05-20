"use client";

/**
 * LoadKisiKisiSheet (ADR-0012 inline rewrite) — replaces the old
 * apply-template right-pull-sheet. Centered modal with three options
 * presented as cards:
 *   - "Pilih dari Template" → expands to a template list + Apply CTA
 *   - "Generate dari Soal" → calls the parent's reverse-flow handler
 *     (typically opens AI chat with a primed prompt). Disabled when
 *     the exam has no questions yet.
 *   - Cancel
 *
 * The sheet itself does the clone API call so the parent only needs to
 * pass examId + a reload trigger.
 */

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import {
  cloneBlueprintToExam,
  listBlueprintTemplates,
  type BlueprintTemplate,
} from "@/lib/modules-api";
import { useToast } from "@/components/ui/toast";
import {
  ArrowLeft,
  ChevronRight,
  Layers,
  Loader2,
  Sparkles,
  X,
} from "lucide-react";
import { cn } from "@/lib/cn";

export interface LoadKisiKisiSheetProps {
  open: boolean;
  examId: string;
  /** True when the exam already has a blueprint loaded — title flips to
   *  "Manage Kisi-Kisi" and the warning about replacement shows. */
  hasBlueprint: boolean;
  /** True when the exam has questions — enables the "Generate dari soal"
   *  card and surfaces the secondary description. */
  hasQuestions: boolean;
  /** Default tab to surface when opening (e.g. "templates" when no
   *  blueprint is loaded; "manage" when one is). */
  defaultTab?: "templates" | "generate";
  onClose: () => void;
  /** Called after a successful template apply. */
  onApplied: () => void;
  /** Called when user picks "Generate dari Soal". Parent handles AI
   *  chat or reverse-flow trigger. */
  onGenerateFromQuestions?: () => void;
}

type View = "menu" | "templates";

export function LoadKisiKisiSheet({
  open,
  examId,
  hasBlueprint,
  hasQuestions,
  defaultTab,
  onClose,
  onApplied,
  onGenerateFromQuestions,
}: LoadKisiKisiSheetProps) {
  const { toast } = useToast();
  const [view, setView] = useState<View>(
    defaultTab === "templates" ? "templates" : "menu",
  );
  const [templates, setTemplates] = useState<BlueprintTemplate[]>([]);
  const [loadingList, setLoadingList] = useState(false);
  const [selectedTpl, setSelectedTpl] = useState<string>("");
  const [cloning, setCloning] = useState(false);

  useEffect(() => {
    if (!open) return;
    setView(defaultTab === "templates" ? "templates" : "menu");
    setSelectedTpl("");
  }, [open, defaultTab]);

  useEffect(() => {
    if (!open || view !== "templates" || templates.length > 0) return;
    let cancelled = false;
    setLoadingList(true);
    listBlueprintTemplates({}).then((res) => {
      if (cancelled) return;
      if (res.data) {
        // Allow both draft and published templates per user request —
        // teachers iterate on a draft template often, applying it to
        // a sandbox exam to verify before publishing.
        setTemplates(
          res.data.data.filter(
            (t) => t.canAccess && t.status !== "archived",
          ),
        );
      }
      setLoadingList(false);
    });
    return () => {
      cancelled = true;
    };
  }, [open, view, templates.length]);

  // Esc closes the modal.
  useEffect(() => {
    if (!open) return;
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [open, onClose]);

  async function handleApply() {
    if (!selectedTpl) {
      toast({
        tone: "error",
        title: "Pilih template dulu",
      });
      return;
    }
    setCloning(true);
    const res = await cloneBlueprintToExam(examId, {
      templateId: selectedTpl,
      replace: hasBlueprint,
    });
    setCloning(false);
    if (res.error) {
      toast({
        tone: "error",
        title: "Apply failed",
        description: res.error.message,
      });
      return;
    }
    toast({ tone: "success", title: "Blueprint applied" });
    onApplied();
    onClose();
  }

  // Portal to <body> so the modal escapes any ancestor containing block
  // (e.g. backdrop-filter on PageShell's sticky header would otherwise clip
  // a position:fixed modal to the header strip).
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);

  if (!open || !mounted) return null;

  const title = hasBlueprint ? "Manage Kisi-Kisi" : "Load Kisi-Kisi";

  return createPortal(
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/40 backdrop-blur-[2px] p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="load-kk-title"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="w-full max-w-lg overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--card)] shadow-xl">
        {/* Header */}
        <div className="flex items-start justify-between gap-3 border-b border-[var(--border)] bg-[var(--accent)]/40 px-5 py-3">
          <div className="flex items-center gap-2">
            {view === "templates" && (
              <button
                type="button"
                onClick={() => setView("menu")}
                aria-label="Kembali"
                className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
              >
                <ArrowLeft size={13} />
              </button>
            )}
            <div>
              <h3
                id="load-kk-title"
                className="text-[14px] font-semibold text-[var(--foreground)]"
              >
                {view === "templates" ? "Pilih template" : title}
              </h3>
              <p className="mt-0.5 text-[11px] text-[var(--muted-foreground)]">
                {view === "templates"
                  ? "Hanya template yang sudah dipublish + bisa kamu akses."
                  : hasBlueprint
                    ? "Ganti template, regenerasi, atau lepas binding."
                    : "Pilih cara untuk menyusun kisi-kisi exam ini."}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Tutup"
            className="-mr-1 flex h-7 w-7 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
          >
            <X size={13} />
          </button>
        </div>

        {/* Body */}
        <div className="space-y-2 p-4">
          {view === "menu" ? (
            <>
              {hasBlueprint && (
                <p className="rounded-md border border-[var(--warning)] bg-[var(--warning-soft)] px-3 py-2 text-[10.5px] text-[var(--warning)]">
                  Mengganti template akan menghapus slot yang ada. Soal yang
                  tertaut akan kehilangan link slot dan butuh re-link manual.
                </p>
              )}
              <OptionCard
                icon={<Layers size={14} />}
                title={hasBlueprint ? "Pilih template lain" : "Pilih dari Template"}
                description="Browse blueprint published. Slot, kompetensi, dan AKM dimensions auto-clone."
                onClick={() => setView("templates")}
              />
              <p className="px-1 pt-1 text-[10.5px] text-[var(--muted-foreground)]">
                Belum siap pakai template? Tutup dialog dan langsung tulis soal
                — tiap soal otomatis bikin slot kisi-kisi sendiri.
              </p>
            </>
          ) : (
            <TemplateList
              templates={templates}
              loading={loadingList}
              selected={selectedTpl}
              onSelect={setSelectedTpl}
            />
          )}
        </div>

        {/* Footer */}
        {view === "templates" && (
          <div className="flex items-center justify-end gap-2 border-t border-[var(--border)] bg-[var(--accent)]/30 px-4 py-3">
            <button
              type="button"
              onClick={onClose}
              disabled={cloning}
              className="h-8 rounded-md px-3 text-[11.5px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] disabled:opacity-50 transition-colors"
            >
              Batal
            </button>
            <button
              type="button"
              onClick={handleApply}
              disabled={!selectedTpl || cloning}
              className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[var(--primary)] px-3 text-[11.5px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 disabled:opacity-50 transition-all"
            >
              {cloning ? (
                <Loader2 size={11} className="animate-spin" />
              ) : (
                <Sparkles size={11} />
              )}
              {hasBlueprint ? "Replace" : "Apply"}
            </button>
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}

function OptionCard({
  icon,
  title,
  description,
  onClick,
  disabled,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "flex w-full items-start gap-3 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2.5 text-left transition-all",
        disabled
          ? "opacity-50 cursor-not-allowed"
          : "hover:border-[var(--brand)]/40 hover:bg-[var(--brand-soft)]/40 active:scale-[0.99]",
      )}
    >
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-[var(--brand-soft)] text-[var(--brand)]">
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-[12.5px] font-semibold text-[var(--foreground)]">
          {title}
        </p>
        <p className="mt-0.5 text-[11px] text-[var(--muted-foreground)]">
          {description}
        </p>
      </div>
      <ChevronRight
        size={13}
        className="mt-2 shrink-0 text-[var(--muted-foreground)]"
      />
    </button>
  );
}

function TemplateList({
  templates,
  loading,
  selected,
  onSelect,
}: {
  templates: BlueprintTemplate[];
  loading: boolean;
  selected: string;
  onSelect: (id: string) => void;
}) {
  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 size={14} className="animate-spin text-[var(--muted-foreground)]" />
      </div>
    );
  }
  if (templates.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-[var(--border-strong)] bg-[var(--accent)]/40 p-6 text-center">
        <p className="text-[12px] text-[var(--muted-foreground)]">
          Belum ada blueprint published.
        </p>
        <a
          href="/app/blueprints"
          className="mt-2 inline-flex text-[11px] font-semibold text-[var(--brand)] hover:underline"
        >
          Buka library blueprint →
        </a>
      </div>
    );
  }
  return (
    <div className="max-h-80 space-y-1.5 overflow-y-auto pr-1">
      {templates.map((t) => {
        const isSelected = selected === t.id;
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onSelect(t.id)}
            className={cn(
              "flex w-full items-start gap-3 rounded-lg border px-3 py-2.5 text-left transition-all",
              isSelected
                ? "border-[var(--brand)] bg-[var(--brand-soft)]/40"
                : "border-[var(--border)] bg-[var(--background)] hover:border-[var(--brand)]/40 hover:bg-[var(--brand-soft)]/30",
            )}
          >
            <div className="flex-1 min-w-0">
              <p className="text-[12.5px] font-semibold text-[var(--foreground)]">
                {t.title}
              </p>
              <p className="mt-0.5 text-[10.5px] text-[var(--muted-foreground)]">
                {t.curriculumCode.toUpperCase()} ·{" "}
                {t.blueprintType.replace("akm_", "AKM ")} · {t.totalSlots} slot
                · {t.totalPoints} pts
                {t.strictCoverage ? " · strict" : ""}
              </p>
            </div>
            {isSelected && (
              <span className="rounded-md bg-[var(--brand)] px-1.5 py-0.5 text-[9.5px] font-semibold text-white">
                Dipilih
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
