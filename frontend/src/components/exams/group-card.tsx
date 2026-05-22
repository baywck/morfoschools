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
import { RichEditor } from "@/components/ui/rich-editor";
import { InputField } from "@/components/ui/input-field";
import { isHtmlContent } from "@/components/ui/rendered-content";

// Convert plain-text stimulus content (legacy or AI-generated
// without HTML tags) into TipTap-friendly paragraph HTML so the
// editor preserves newlines and the rendered output matches.
// RenderedContent already handles plain text via white-space:
// pre-wrap, but TipTap collapses whitespace inside its DOM —
// the editor view would show one giant blob without this.
function normalizeStimulusForEditor(raw: string): string {
  if (!raw) return "";
  if (isHtmlContent(raw)) return raw;
  const escaped = raw
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
  // Treat double-newline as paragraph break, single newline as <br>
  // so the editor mirrors what RenderedContent shows.
  return escaped
    .split(/\n{2,}/)
    .map((para) => `<p>${para.replace(/\n/g, "<br>")}</p>`)
    .join("");
}
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

  // Inline editor state — lets the user edit the group's stimulus
  // snapshot directly without round-tripping through the master
  // stimuli row. Library import via StimulusPicker is still available
  // as an explicit action for power users.
  const [editTitle, setEditTitle] = useState(group.stimulusTitleSnapshot ?? "");
  const [editBody, setEditBody] = useState(
    normalizeStimulusForEditor(group.stimulusBodySnapshot ?? "")
  );
  const [showLibrary, setShowLibrary] = useState(false);

  // Sync editor state when the group prop changes (after parent
  // refetch following save). Without this the editor keeps showing
  // stale values when the canonical group data is refreshed.
  useEffect(() => {
    setEditTitle(group.stimulusTitleSnapshot ?? "");
    setEditBody(normalizeStimulusForEditor(group.stimulusBodySnapshot ?? ""));
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

  async function handleSaveSnapshot() {
    setSavingStim(true);
    const res = await updateQuestionGroup(group.id, {
      titleSnapshot: editTitle,
      bodySnapshot: editBody,
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
    toast({ tone: "success", title: "Stimulus disimpan" });
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

      {/* Stimulus inline editor (collapsible). When the group body
          is collapsed this is also hidden via the bodyOpen effect. */}
      {stimulusOpen && canEdit && bodyOpen && (
        <div className="space-y-3 border-b border-[var(--border)] bg-[var(--background)] p-3">
          <InputField
            label="Judul stimulus"
            value={editTitle}
            onChange={(e) => setEditTitle(e.target.value)}
          />
          <div>
            <label className="mb-1.5 block text-[10.5px] font-semibold uppercase tracking-wider text-[var(--muted-foreground)]">
              Isi stimulus
            </label>
            <RichEditor value={editBody} onChange={setEditBody} />
          </div>
          <div className="flex items-center justify-between gap-2">
            <button
              type="button"
              onClick={() => setShowLibrary((v) => !v)}
              className="text-[10.5px] font-medium text-[var(--muted-foreground)] hover:text-[var(--brand)] transition-colors"
            >
              {showLibrary ? "Sembunyikan library" : "Pilih dari library…"}
            </button>
            <div className="flex items-center gap-2">
              {group.stimulusId && (
                <button
                  type="button"
                  onClick={handleClearStimulus}
                  className="h-8 rounded-md border border-[var(--border)] bg-[var(--background)] px-3 text-[11px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--destructive-soft)] hover:text-[var(--destructive)] transition-colors"
                >
                  Hapus stimulus
                </button>
              )}
              <button
                type="button"
                onClick={() => setStimulusOpen(false)}
                className="h-8 rounded-md border border-[var(--border)] bg-[var(--background)] px-3 text-[11px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={handleSaveSnapshot}
                disabled={savingStim}
                className="flex h-8 items-center gap-1 rounded-md bg-[var(--brand)] px-3 text-[11px] font-semibold text-white hover:opacity-90 disabled:opacity-50 transition-all"
              >
                {savingStim && <Loader2 size={11} className="animate-spin" />}
                Simpan
              </button>
            </div>
          </div>
          {showLibrary && (
            <div className="rounded-md border border-dashed border-[var(--border)] p-2">
              <p className="mb-1.5 text-[10px] text-[var(--muted-foreground)]">
                Power user: import passage dari library bersama (akan menimpa
                judul + isi di atas).
              </p>
              <StimulusPicker
                value={group.stimulusId ?? null}
                onSelect={(s) => {
                  setEditTitle(s.title);
                  setEditBody(normalizeStimulusForEditor(s.content));
                  setShowLibrary(false);
                  void handleSelectStimulus(s);
                }}
              />
            </div>
          )}
        </div>
      )}

      {/* Body — child questions + footer */}
      {bodyOpen && (
        <div className="space-y-1.5 p-2">
          {/* Render the stimulus body inline above the questions when
              not in edit mode and there's actual content to read. */}
          {!stimulusOpen && stimulusBody && (
            <div className="mb-2 rounded-lg border border-[var(--border)] bg-[var(--accent)]/30 px-3 py-2.5">
              <div className="text-[11.5px] leading-relaxed text-[var(--foreground)] [&_p]:mb-1.5 last:[&_p]:mb-0">
                <RenderedContent html={stimulusBody} />
              </div>
            </div>
          )}
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
