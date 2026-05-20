"use client";

/**
 * GroupCard (ADR-0012 inline rewrite) — visual cluster for questions
 * that share a stimulus. The card header is the visual anchor for the
 * group's stimulus; clicking it opens an inline picker. Below the
 * header is a sortable list of question accordions where each child
 * question's per-question stimulus picker is suppressed (group's
 * stimulus is the source of truth).
 *
 * The card itself is sortable in its parent context (root or section).
 */

import { useState } from "react";
import {
  ChevronDown,
  ChevronRight,
  GripVertical,
  Layers,
  Loader2,
  Plus,
  Trash2,
} from "lucide-react";
import {
  updateQuestionGroup,
  deleteQuestionGroup,
  type ExamQuestionGroup,
  type Stimulus,
} from "@/lib/modules-api";
import { useToast } from "@/components/ui/toast";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RenderedContent, stripHtmlPreview } from "@/components/ui/rendered-content";
import { StimulusPicker } from "@/components/exams/stimulus-picker";
import { cn } from "@/lib/cn";

export interface GroupCardProps {
  group: ExamQuestionGroup;
  /** Total questions in this group (rendered as the count badge). */
  questionCount: number;
  /** Children render slot — usually <SortableContext> + <QuestionAccordion>s. */
  children: React.ReactNode;
  canEdit: boolean;
  /** Add-question handler scoped to this group. */
  onAddQuestion: () => void;
  /** Called after stimulus update / delete to trigger parent reload. */
  onChange: () => void;
  /** Drag handle props for the group itself (when DnD enabled). */
  dragHandleProps?: React.HTMLAttributes<HTMLButtonElement> & {
    ref?: (node: HTMLButtonElement | null) => void;
  };
}

export function GroupCard({
  group,
  questionCount,
  children,
  canEdit,
  onAddQuestion,
  onChange,
  dragHandleProps,
}: GroupCardProps) {
  const { toast } = useToast();
  const [stimulusOpen, setStimulusOpen] = useState(false);
  const [bodyOpen, setBodyOpen] = useState(true);
  const [savingStim, setSavingStim] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleSelectStimulus(s: Stimulus) {
    setSavingStim(true);
    const res = await updateQuestionGroup(group.id, { stimulusId: s.id });
    setSavingStim(false);
    if (res.error) {
      toast({
        tone: "error",
        title: "Gagal mengikat stimulus",
        description: res.error.message,
      });
      return;
    }
    toast({ tone: "success", title: "Stimulus terikat ke group" });
    setStimulusOpen(false);
    onChange();
  }

  async function handleClearStimulus() {
    setSavingStim(true);
    const res = await updateQuestionGroup(group.id, { stimulusId: "" });
    setSavingStim(false);
    if (res.error) {
      toast({
        tone: "error",
        title: "Gagal menghapus stimulus",
        description: res.error.message,
      });
      return;
    }
    toast({ tone: "success", title: "Stimulus dilepas" });
    setStimulusOpen(false);
    onChange();
  }

  async function handleDelete() {
    setDeleting(true);
    const res = await deleteQuestionGroup(group.id);
    setDeleting(false);
    if (res.error) {
      toast({
        tone: "error",
        title: "Gagal hapus group",
        description: res.error.message,
      });
      return;
    }
    toast({
      tone: "success",
      title: "Group dihapus",
      description: "Soal di dalam group dilepaskan ke parent.",
    });
    setConfirmDelete(false);
    onChange();
  }

  const stimulusTitle = group.stimulusTitleSnapshot ?? null;
  const stimulusBody = group.stimulusBodySnapshot ?? null;

  return (
    <div className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)]">
      {/* Header strip */}
      <div className="flex items-center gap-2 border-b border-[var(--border)] bg-[var(--accent)]/40 px-3 py-2">
        {dragHandleProps && canEdit && (
          <button
            type="button"
            {...dragHandleProps}
            aria-label="Drag untuk pindahkan group"
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] cursor-grab active:cursor-grabbing transition-colors"
          >
            <GripVertical size={13} />
          </button>
        )}
        <button
          type="button"
          onClick={() => setBodyOpen((v) => !v)}
          className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
          aria-label={bodyOpen ? "Collapse group" : "Expand group"}
          aria-expanded={bodyOpen}
        >
          {bodyOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
        </button>
        <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-[var(--brand-soft)] text-[var(--brand)]">
          <Layers size={11} />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-[12px] font-semibold text-[var(--foreground)]">
            {stimulusTitle ?? "Stimulus group"}
          </p>
          <p className="text-[10px] text-[var(--muted-foreground)] truncate">
            {stimulusTitle
              ? stimulusBody
                ? stripHtmlPreview(stimulusBody, 90)
                : "Tanpa isi"
              : "Belum ada stimulus — klik untuk pilih"}
            {" · "}
            {questionCount} soal
          </p>
        </div>
        {canEdit && (
          <>
            <button
              type="button"
              onClick={() => setStimulusOpen((v) => !v)}
              className={cn(
                "inline-flex h-7 items-center gap-1 rounded-md px-2 text-[10.5px] font-medium transition-colors",
                stimulusTitle
                  ? "border border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]"
                  : "bg-[var(--brand-soft)] text-[var(--brand)] hover:opacity-90",
              )}
            >
              {savingStim && <Loader2 size={10} className="animate-spin" />}
              {stimulusTitle ? "Edit stimulus" : "+ Tambah stimulus"}
            </button>
            <button
              type="button"
              onClick={() => setConfirmDelete(true)}
              aria-label="Hapus group"
              className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--destructive-soft)] hover:text-[var(--destructive)] transition-colors"
            >
              <Trash2 size={12} />
            </button>
          </>
        )}
      </div>

      {/* Stimulus picker (collapsible inline) */}
      {stimulusOpen && canEdit && (
        <div className="border-b border-[var(--border)] bg-[var(--background)] p-3">
          <StimulusPicker
            value={group.stimulusId ?? null}
            onSelect={handleSelectStimulus}
            onClear={group.stimulusId ? handleClearStimulus : undefined}
          />
          {stimulusBody && group.stimulusId && (
            <details className="mt-2 rounded-md border border-[var(--border)] bg-[var(--card)] p-2 text-[11.5px] leading-relaxed text-[var(--foreground)]">
              <summary className="cursor-pointer text-[10.5px] font-semibold text-[var(--muted-foreground)]">
                Snapshot saat ini (preview)
              </summary>
              <div className="mt-1">
                <RenderedContent html={stimulusBody} />
              </div>
            </details>
          )}
        </div>
      )}

      {/* Body — child questions + footer */}
      {bodyOpen && (
        <div className="space-y-1.5 p-2">
          {children}
          {canEdit && (
            <button
              type="button"
              onClick={onAddQuestion}
              className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-[var(--border-strong)] bg-[var(--accent)]/30 px-3 py-2 text-[11px] font-medium text-[var(--muted-foreground)] hover:border-[var(--brand)]/40 hover:bg-[var(--brand-soft)]/40 hover:text-[var(--brand)] transition-colors"
            >
              <Plus size={11} /> Tambah soal ke group
            </button>
          )}
        </div>
      )}

      <ConfirmDialog
        open={confirmDelete}
        title="Hapus group?"
        description="Group dihapus, soal di dalamnya tetap ada tapi tidak lagi tertaut ke stimulus group."
        confirmLabel="Hapus"
        destructive
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
      />
    </div>
  );
}

function truncate(s: string, n: number) {
  return s.length > n ? s.slice(0, n - 1) + "…" : s;
}
void truncate;
