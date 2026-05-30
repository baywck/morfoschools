"use client";

import { ChevronDown, FileText, Loader2, Trash2 } from "lucide-react";
import { InputField } from "@/components/ui/input-field";
import { RichEditor } from "@/components/ui/rich-editor";
import { RenderedContent } from "@/components/ui/rendered-content";
import { StimulusPicker } from "@/components/exams/stimulus-picker";
import { StimulusLibraryModal } from "@/components/exams/stimulus-library-modal";
import type { Stimulus } from "@/lib/modules-api";
import { cn } from "@/lib/cn";

export interface StimulusEditorPanelProps {
  canEdit: boolean;
  open: boolean;
  stimulusId: string | null;
  title: string | null;
  body: string;
  saving: boolean;
  saveToLibrary: boolean;
  showLibrary: boolean;
  onToggle: () => void;
  onTitleChange: (value: string) => void;
  onBodyChange: (value: string) => void;
  onSaveToLibraryChange: (value: boolean) => void;
  onSave: () => void;
  onDelete: () => void;
  deleteLabel?: string;
  onSelect: (stimulus: Stimulus) => void;
  onClear: () => void;
  onOpenLibrary: () => void;
  onCloseLibrary: () => void;
  onSelectLibrary: (stimulus: Stimulus) => void;
}

export function StimulusEditorPanel({
  canEdit,
  open,
  stimulusId,
  title,
  body,
  saving,
  saveToLibrary,
  showLibrary,
  onToggle,
  onTitleChange,
  onBodyChange,
  onSaveToLibraryChange,
  onSave,
  onDelete,
  deleteLabel = "Hapus stimulus",
  onSelect,
  onClear,
  onOpenLibrary,
  onCloseLibrary,
  onSelectLibrary,
}: StimulusEditorPanelProps) {
  const hasStimulus = !!stimulusId;

  return (
    <div
      className={cn(
        "overflow-hidden rounded-lg border bg-[var(--background)]",
        hasStimulus ? "border-[var(--brand)]/35 shadow-sm" : "border-dashed border-[var(--border-strong)]",
      )}
    >
      <div className="flex items-center justify-between gap-2 border-b border-[var(--border)] bg-[var(--accent)]/20 px-3 py-2.5">
        <button
          type="button"
          onClick={onToggle}
          className="flex min-w-0 flex-1 items-center gap-2 text-left"
        >
          <FileText size={12} className={hasStimulus ? "text-[var(--brand)]" : "text-[var(--muted-foreground)]"} />
          <div className="min-w-0 flex-1">
            <p className="text-[10.5px] font-semibold uppercase tracking-wider text-[var(--muted-foreground)]">
              Stimulus
            </p>
            <p className="truncate text-[11.5px] font-semibold text-[var(--foreground)]">
              {title || (hasStimulus ? "Tanpa judul" : "Belum ada stimulus")}
            </p>
          </div>
        </button>

        <div className="flex shrink-0 items-center gap-1.5">
          {open && hasStimulus && (
            <button
              type="button"
              onClick={onOpenLibrary}
              className="inline-flex h-7 items-center gap-1 rounded-md border border-[var(--border)] bg-[var(--background)] px-2.5 text-[10.5px] font-medium text-[var(--muted-foreground)] transition-colors hover:border-[var(--brand)]/40 hover:bg-[var(--brand-soft)] hover:text-[var(--brand)]"
            >
              📚 Pilih dari library
            </button>
          )}
          <button
            type="button"
            onClick={onToggle}
            className={cn(
              "inline-flex h-7 items-center gap-1 rounded-md border px-2.5 text-[10.5px] font-semibold shadow-sm transition-colors",
              open
                ? "border-[var(--border)] bg-[var(--muted)] text-[var(--foreground)] hover:bg-[var(--muted)]/80"
                : "border-[var(--border)] bg-[var(--card)] text-[var(--muted-foreground)] hover:border-[var(--brand)]/40 hover:bg-[var(--brand-soft)] hover:text-[var(--brand)]",
            )}
          >
            <ChevronDown size={10} className={cn("transition-transform", !open && "-rotate-90")} />
            {open ? "Cancel" : hasStimulus ? "Edit" : "Tambah"}
          </button>
        </div>
      </div>

      {hasStimulus && !open && (
        <div className="bg-[var(--accent)]/20 px-3 pb-3 pt-2">
          {body ? (
            <div className="max-h-32 overflow-hidden rounded-md border border-[var(--border)] bg-[var(--card)] px-3 py-2 text-[11.5px] leading-relaxed text-[var(--foreground)]">
              <RenderedContent html={body} className="text-[11.5px] [&_p]:my-1" />
            </div>
          ) : (
            <p className="rounded-md border border-[var(--border)] bg-[var(--card)] px-3 py-2 text-[11px] text-[var(--muted-foreground)]">
              Memuat stimulus…
            </p>
          )}
        </div>
      )}

      {open && (
        <div className="bg-[var(--background)]">
          {hasStimulus ? (
            <>
              <div className="space-y-4 px-4 py-4">
                <InputField
                  label="Judul stimulus"
                  value={title ?? ""}
                  onChange={(e) => onTitleChange(e.target.value)}
                  disabled={!canEdit}
                />
                <div>
                  <label className="mb-1.5 block text-[10.5px] font-semibold uppercase tracking-wider text-[var(--muted-foreground)]">
                    Isi stimulus
                  </label>
                  <RichEditor
                    value={body}
                    onChange={onBodyChange}
                    minRows={5}
                    placeholder="Bacaan, kasus, atau materi pendukung."
                    disabled={!canEdit}
                    ariaLabel="Isi stimulus"
                  />
                </div>

                <label
                  className={cn(
                    "flex items-start gap-3 rounded-lg border px-3 py-2.5 cursor-pointer transition-colors select-none",
                    saveToLibrary
                      ? "border-[var(--brand)]/40 bg-[var(--brand-soft)]/40"
                      : "border-[var(--border)] bg-[var(--accent)]/20 hover:bg-[var(--accent)]/40",
                  )}
                >
                  <input
                    type="checkbox"
                    checked={saveToLibrary}
                    onChange={(e) => onSaveToLibraryChange(e.target.checked)}
                    className="mt-0.5 h-4 w-4 shrink-0 rounded border-[var(--border)] text-[var(--brand)] focus:ring-2 focus:ring-[var(--brand)]/40"
                  />
                  <div className="flex-1">
                    <p className="text-[12px] font-semibold text-[var(--foreground)]">Simpan ke library bersama</p>
                    <p className="mt-0.5 text-[10.5px] leading-relaxed text-[var(--muted-foreground)]">
                      Stimulus ini bisa dipakai ulang di exam lain. Tanpa centang ini, stimulus tetap untuk soal ini.
                    </p>
                  </div>
                </label>
              </div>

              <div className="flex items-center justify-between gap-2 border-t border-[var(--border)] bg-[var(--accent)]/20 px-4 py-2.5">
                <button
                  type="button"
                  onClick={onDelete}
                  disabled={saving || !canEdit}
                  className="inline-flex h-8 items-center gap-1 rounded-md px-2.5 text-[11px] font-medium text-[var(--muted-foreground)] transition-colors hover:bg-[var(--destructive-soft)] hover:text-[var(--destructive)] disabled:opacity-50"
                >
                  <Trash2 size={11} /> {deleteLabel}
                </button>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={onSave}
                    disabled={saving || !canEdit}
                    className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[var(--brand)] px-3.5 text-[11px] font-semibold text-white shadow-sm transition-opacity hover:opacity-90 disabled:opacity-50"
                  >
                    {saving && <Loader2 size={11} className="animate-spin" />}
                    Simpan
                  </button>
                </div>
              </div>
            </>
          ) : (
            <div className="px-3 pb-3 pt-3">
              <StimulusPicker value={stimulusId} onSelect={onSelect} onClear={onClear} />
            </div>
          )}
        </div>
      )}

      {showLibrary && (
        <StimulusLibraryModal open={showLibrary} onClose={onCloseLibrary} onSelect={onSelectLibrary} />
      )}
    </div>
  );
}
