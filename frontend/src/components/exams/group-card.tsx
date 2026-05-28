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

import { useEffect, useState } from "react";
import {
  ChevronDown,
  ChevronRight,
  GripVertical,
  Layers,
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
import { stripHtmlPreview } from "@/components/ui/rendered-content";
import { StimulusEditorPanel } from "@/components/exams/stimulus-editor-panel";
import { normalizeRichTextForEditor } from "@/lib/rich-text";
import { InlineMagicPopover } from "@/components/ai/inline-magic-popover";
import { cn } from "@/lib/cn";

export interface GroupCardProps {
  group: ExamQuestionGroup;
  /** Total questions in this group (rendered as the count badge). */
  questionCount: number;
  /** Children render slot — usually <SortableContext> + <QuestionAccordion>s. */
  children: React.ReactNode;
  canEdit: boolean;
  /** Exam ID needed for the inline magic AI popover. Passed down
   *  from ExamCanvas so the AI knows the parent exam without
   *  another lookup. */
  examId?: string;
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
  examId,
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

  // Inline editor state — lets the user edit the group's stimulus
  // snapshot directly without round-tripping through the master
  // stimuli row. Library import via StimulusPicker is still available
  // as an explicit action for power users.
  const [editTitle, setEditTitle] = useState(group.stimulusTitleSnapshot ?? "");
  const [editBody, setEditBody] = useState(
    normalizeRichTextForEditor(group.stimulusBodySnapshot)
  );
  const [showLibrary, setShowLibrary] = useState(false);

  // Sync editor state when the group prop changes (after parent
  // refetch following save). Without this the editor keeps showing
  // stale values when the canonical group data is refreshed.
  useEffect(() => {
    setEditTitle(group.stimulusTitleSnapshot ?? "");
    setEditBody(normalizeRichTextForEditor(group.stimulusBodySnapshot));
  }, [group.stimulusTitleSnapshot, group.stimulusBodySnapshot, group.id]);

  // When the group body is collapsed, also hide the stimulus editor
  // so the user has one consistent way to fold the entire group.
  useEffect(() => {
    if (!bodyOpen) setStimulusOpen(false);
  }, [bodyOpen]);

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

  // saveToLibrary checkbox state. Initialized true when the group is
  // already linked to a shared stimulus (so the toggle reflects current
  // status); otherwise false (default exam_scoped per Opsi B).
  const isCurrentlyShared = group.stimulusId != null && group.groupType === "stimulus" && (group as { stimulusLifecycle?: string }).stimulusLifecycle === "shared";
  const [saveToLibrary, setSaveToLibrary] = useState(isCurrentlyShared);

  async function handleSaveSnapshot() {
    setSavingStim(true);
    const res = await updateQuestionGroup(group.id, {
      titleSnapshot: editTitle,
      bodySnapshot: editBody,
      saveToLibrary,
    });
    setSavingStim(false);
    if (res.error) {
      toast({
        tone: "error",
        title: "Gagal simpan stimulus",
        description: res.error.message,
      });
      return;
    }
    toast({
      tone: "success",
      title: "Stimulus disimpan",
      description: saveToLibrary ? "Tersimpan juga ke library bersama." : undefined,
    });
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
            <InlineMagicPopover
              entityKind="group"
              entityId={group.id}
              examId={examId}
              className="shrink-0"
            />
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

      {canEdit && bodyOpen && (
        <div className="border-b border-[var(--border)] bg-[var(--background)] p-2">
          <StimulusEditorPanel
            canEdit={canEdit}
            open={stimulusOpen}
            stimulusId={group.stimulusId ?? null}
            title={editTitle}
            body={editBody}
            saving={savingStim}
            saveToLibrary={saveToLibrary}
            showLibrary={showLibrary}
            deleteLabel="Hapus stimulus"
            onToggle={() => setStimulusOpen((v) => !v)}
            onTitleChange={setEditTitle}
            onBodyChange={setEditBody}
            onSaveToLibraryChange={setSaveToLibrary}
            onSave={handleSaveSnapshot}
            onDelete={handleClearStimulus}
            onSelect={handleSelectStimulus}
            onClear={handleClearStimulus}
            onOpenLibrary={() => setShowLibrary(true)}
            onCloseLibrary={() => setShowLibrary(false)}
            onSelectLibrary={(s) => {
              setShowLibrary(false);
              setEditTitle(s.title);
              setEditBody(normalizeRichTextForEditor(s.content));
            }}
          />
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
