"use client";

/**
 * SectionCard (Phase 9.8 rewrite) — mandatory container for exam
 * questions. Header strip carries an inline-editable title, a delete
 * trash icon (disabled when this is the only section), and the body
 * hosts a sortable area for groups + standalone questions plus a
 * footer row with the two add-buttons.
 *
 * Section deletion reassigns members to the exam's first remaining
 * section so the NOT NULL invariant on exam_questions.section_id
 * holds. The frontend only initiates the delete; the backend handles
 * the reassignment in one transaction.
 */

import { useEffect, useRef, useState } from "react";
import {
  ChevronDown,
  ChevronRight,
  GripVertical,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import {
  updateExamSection,
  deleteExamSection,
  type ExamSection,
} from "@/lib/modules-api";
import { useToast } from "@/components/ui/toast";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { cn } from "@/lib/cn";

export interface SectionCardProps {
  section: ExamSection;
  /** Sortable area children (groups + standalone questions). */
  children: React.ReactNode;
  questionCount: number;
  canEdit: boolean;
  /** When true the trash icon is disabled (cannot delete last section). */
  isOnlySection: boolean;
  onAddQuestion: () => void;
  onAddGroup: () => void;
  onChange: () => void;
  dragHandleProps?: React.HTMLAttributes<HTMLButtonElement> & {
    ref?: (node: HTMLButtonElement | null) => void;
  };
}

export function SectionCard({
  section,
  children,
  questionCount,
  canEdit,
  isOnlySection,
  onAddQuestion,
  onAddGroup,
  onChange,
  dragHandleProps,
}: SectionCardProps) {
  const { toast } = useToast();
  const [bodyOpen, setBodyOpen] = useState(true);
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(section.title);
  const [savingTitle, setSavingTitle] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setTitle(section.title);
  }, [section.title]);

  useEffect(() => {
    if (editing) inputRef.current?.focus();
  }, [editing]);

  async function handleSaveTitle() {
    const trimmed = title.trim();
    if (!trimmed) {
      setTitle(section.title);
      setEditing(false);
      return;
    }
    if (trimmed === section.title) {
      setEditing(false);
      return;
    }
    setSavingTitle(true);
    const res = await updateExamSection(section.id, { title: trimmed });
    setSavingTitle(false);
    if (res.error) {
      toast({
        tone: "error",
        title: "Gagal mengubah judul",
        description: res.error.message,
      });
      setTitle(section.title);
      return;
    }
    toast({ tone: "success", title: "Section diperbarui" });
    setEditing(false);
    onChange();
  }

  async function handleDelete() {
    setDeleting(true);
    const res = await deleteExamSection(section.id);
    setDeleting(false);
    if (res.error) {
      const fieldErr = res.error.fields?.section;
      toast({
        tone: "error",
        title: "Gagal menghapus section",
        description: fieldErr ?? res.error.message,
      });
      return;
    }
    toast({
      tone: "success",
      title: "Section dihapus",
      description: "Soal di dalamnya dipindah ke section pertama.",
    });
    setConfirmDelete(false);
    onChange();
  }

  return (
    <div className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)]">
      {/* Header strip */}
      <div className="flex items-center gap-2 border-b border-[var(--border)] bg-[var(--accent)] px-3 py-2.5">
        {dragHandleProps && canEdit && (
          <button
            type="button"
            {...dragHandleProps}
            aria-label="Drag untuk pindahkan section"
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] cursor-grab active:cursor-grabbing transition-colors"
          >
            <GripVertical size={13} />
          </button>
        )}
        <button
          type="button"
          onClick={() => setBodyOpen((v) => !v)}
          aria-label={bodyOpen ? "Collapse section" : "Expand section"}
          aria-expanded={bodyOpen}
          className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
        >
          {bodyOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
        </button>
        <div className="flex-1 min-w-0">
          {editing ? (
            <input
              ref={inputRef}
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              onBlur={handleSaveTitle}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSaveTitle();
                if (e.key === "Escape") {
                  setTitle(section.title);
                  setEditing(false);
                }
              }}
              className="w-full rounded-md border border-[var(--brand)] bg-[var(--card)] px-2 py-1 text-[12.5px] font-semibold text-[var(--foreground)] outline-none focus:ring-2 focus:ring-[var(--field-ring)]"
              aria-label="Edit judul section"
            />
          ) : (
            <div className="flex items-center gap-1.5">
              <span className="text-[12.5px] font-semibold text-[var(--foreground)] truncate">
                {section.title}
              </span>
              {canEdit && (
                <button
                  type="button"
                  onClick={() => setEditing(true)}
                  aria-label="Edit judul section"
                  className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--brand)] transition-colors"
                >
                  <Pencil size={11} />
                </button>
              )}
            </div>
          )}
          <p className="text-[10px] text-[var(--muted-foreground)]">
            {questionCount} soal
            {savingTitle && " · menyimpan..."}
          </p>
        </div>
        {canEdit && (
          <button
            type="button"
            onClick={() => !isOnlySection && setConfirmDelete(true)}
            disabled={isOnlySection}
            aria-label={
              isOnlySection ? "Tidak bisa hapus section terakhir" : "Hapus section"
            }
            title={
              isOnlySection
                ? "Tidak bisa hapus section terakhir. Setiap exam butuh minimal satu section."
                : "Hapus section"
            }
            className={cn(
              "flex h-7 w-7 shrink-0 items-center justify-center rounded-md transition-colors",
              isOnlySection
                ? "cursor-not-allowed text-[var(--muted-foreground)]/40"
                : "text-[var(--muted-foreground)] hover:bg-[var(--destructive-soft)] hover:text-[var(--destructive)]",
            )}
          >
            <Trash2 size={12} />
          </button>
        )}
      </div>

      {/* Body */}
      {bodyOpen && (
        <div className="space-y-1.5 p-2">
          {children}
          {canEdit && (
            <div className="flex flex-col gap-1.5 pt-1 sm:flex-row">
              <button
                type="button"
                onClick={onAddQuestion}
                className="inline-flex h-10 flex-1 items-center justify-center gap-1.5 rounded-md border border-dashed border-[var(--border-strong)] bg-[var(--background)] px-3 text-[12px] font-medium text-[var(--muted-foreground)] hover:border-[var(--brand)]/40 hover:bg-[var(--brand-soft)]/40 hover:text-[var(--brand)] transition-colors sm:h-8 sm:text-[11px]"
              >
                <Plus size={13} className="sm:hidden" />
                <Plus size={11} className="hidden sm:inline" /> Tambah Soal
              </button>
              <button
                type="button"
                onClick={onAddGroup}
                className="inline-flex h-10 flex-1 items-center justify-center gap-1.5 rounded-md border border-dashed border-[var(--border-strong)] bg-[var(--background)] px-3 text-[12px] font-medium text-[var(--muted-foreground)] hover:border-[var(--brand)]/40 hover:bg-[var(--brand-soft)]/40 hover:text-[var(--brand)] transition-colors sm:h-8 sm:text-[11px]"
              >
                <Plus size={13} className="sm:hidden" />
                <Plus size={11} className="hidden sm:inline" /> Tambah Group
              </button>
            </div>
          )}
        </div>
      )}

      <ConfirmDialog
        open={confirmDelete}
        title="Hapus section?"
        description={`Section "${section.title}" akan dihapus. Soal di dalamnya pindah ke section pertama.`}
        confirmLabel="Hapus"
        destructive
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
      />
    </div>
  );
}
