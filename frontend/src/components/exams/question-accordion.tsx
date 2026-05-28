"use client";

/**
 * QuestionAccordion (ADR-0012 inline rewrite) — replaces the
 * RightPullSheet question form. Each question is a row that expands
 * into an inline editor when clicked. The collapsed row shows summary;
 * the expanded form lets the user edit content, options, points,
 * stimulus (when not nested in a group), and slot binding metadata.
 *
 * Stimulus placement is contextual:
 *   - When isInsideGroup=true: per-question stimulus picker is hidden;
 *     the group's stimulus is the source of truth.
 *   - When isInsideGroup=false: a collapsible "Stimulus / Bacaan"
 *     subsection is rendered. Click to expand the StimulusPicker.
 *
 * Slot metadata: when the question is bound to a slot the type is
 * locked and the points input is pre-filled from the slot.
 *
 * Saving is optimistic-ish: we set a local saving flag, surface inline
 * errors next to the offending field, and on success the parent
 * triggers a reload (which collapses the accordion via onSave).
 */

import { useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  ChevronDown,
  ChevronRight,
  Circle,
  ClipboardList,
  GripVertical,
  Loader2,
  Lock,
  Plus,
  Sparkles,
  Trash2,
  X,
} from "lucide-react";
import {
  archiveStimulus,
  createQuestion,
  createQuestionFromSlot,
  getStimulus,
  promoteStimulus,
  updateQuestion,
  updateStimulus,
  type CreateQuestionPayload,
  type Question,
  type QuestionOption,
  type QuestionType,
  type ScoringMode,
  type Stimulus,
  type SlotWithQuestion,
} from "@/lib/modules-api";
import { useToast } from "@/components/ui/toast";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { MerdekaKisiKisiFields } from "@/components/blueprint/merdeka-kisi-kisi-fields";
import { RichEditor } from "@/components/ui/rich-editor";
import { stripHtmlPreview } from "@/components/ui/rendered-content";
import { normalizeRichTextForEditor } from "@/lib/rich-text";
import { StimulusEditorPanel } from "@/components/exams/stimulus-editor-panel";
import { InlineMagicPopover } from "@/components/ai/inline-magic-popover";
import { cn } from "@/lib/cn";

const QUESTION_TYPE_OPTIONS: { value: QuestionType; label: string }[] = [
  { value: "multiple_choice", label: "Multiple Choice" },
  { value: "true_false", label: "True / False" },
  { value: "short_answer", label: "Short Answer" },
  { value: "essay", label: "Essay" },
];

const SCORING_MODE_OPTIONS: { value: ScoringMode; label: string }[] = [
  { value: "correct_all", label: "Correct All — must select exactly all correct" },
  { value: "correct_one", label: "Correct One — any correct = full points" },
  { value: "percentage", label: "Percentage — partial credit" },
];

const QUESTION_TYPE_LABEL: Record<string, string> = {
  multiple_choice: "PG",
  true_false: "B/S",
  short_answer: "Isian",
  essay: "Essay",
};

export interface QuestionAccordionProps {
  /** When undefined this is a brand-new draft accordion (filled from slot or blank). */
  question: Question | null;
  /** Slot context when this accordion fills a kisi-kisi slot. */
  slot?: SlotWithQuestion | null;
  examId: string;
  /** When false the accordion renders read-only (published exam, etc.). */
  canEdit: boolean;
  /** True when the parent has expanded this row. */
  isOpen: boolean;
  /** Called when user clicks the row toggle (header). */
  onToggle: () => void;
  /** Called after a successful save. Parent typically reloads + collapses. */
  onSaved: () => void;
  /** Called when user requests delete (existing question only). */
  onDelete?: () => void;
  /** Drag handle props from useSortable (or undefined when DnD disabled). */
  dragHandleProps?: React.HTMLAttributes<HTMLButtonElement> & {
    ref?: (node: HTMLButtonElement | null) => void;
  };
  /** Cosmetic 1-based number prefix shown in the collapsed row. */
  index: number;
  /** True when this question lives inside a group card. Hides the
   *  per-question stimulus subsection (group's stimulus wins). */
  isInsideGroup?: boolean;
  /** Section pre-assignment when creating a new question. */
  defaultSectionId?: string;
  /** Group pre-assignment when creating a new question inside a group card. */
  defaultGroupId?: string;
  /** When the parent forces this accordion into edit mode immediately
   *  (e.g. user just clicked "+ New Question"), this collapses on save
   *  via onCancelDraft. Drafts can also be discarded. */
  isDraft?: boolean;
  onCancelDraft?: () => void;
  /** Phase 9.8: render the inline Kisi-Kisi subsection (CP / Elemen CP / TP /
   *  Materi Pokok / Kelas-Semester / Indikator Soal / level kognitif). The
   *  subsection writes back to the bound slot when present, or rides
   *  along on the create payload so the backend can mint a slot. */
  usesKisiKisi?: boolean;
  /** Legacy compatibility flag; new Kurikulum Merdeka flow ignores AKM-specific labels. */
  /** True when the bound slot was cloned from a template; metadata
   *  fields render read-only with a lock icon. */
  slotLockedFromTemplate?: boolean;
}

export function QuestionAccordion({
  question,
  slot,
  examId,
  canEdit,
  isOpen,
  onToggle,
  onSaved,
  onDelete,
  dragHandleProps,
  index,
  isInsideGroup = false,
  defaultSectionId,
  defaultGroupId,
  isDraft = false,
  onCancelDraft,
  usesKisiKisi = false,
  slotLockedFromTemplate = false,
}: QuestionAccordionProps) {
  const { toast } = useToast();
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  // Local form state. Reset when the underlying question or slot
  // identity changes (e.g. parent reloaded after a save).
  const slotType = (slot?.questionType ?? null) as QuestionType | null;
  const initialType: QuestionType =
    question?.questionType ?? slotType ?? "multiple_choice";
  const [type, setType] = useState<QuestionType>(initialType);
  const [content, setContent] = useState(normalizeRichTextForEditor(question?.content));
  const [explanation, setExplanation] = useState(normalizeRichTextForEditor(question?.explanation));
  const [correctAnswer, setCorrectAnswer] = useState(
    normalizeRichTextForEditor(question?.correctAnswer),
  );
  const [points, setPoints] = useState(
    String(question?.points ?? slot?.points ?? 1),
  );
  const [scoringMode, setScoringMode] = useState<ScoringMode>(
    question?.scoringMode ?? "correct_all",
  );
  const [options, setOptions] = useState<QuestionOption[]>(
    question?.options && question.options.length > 0
      ? question.options
      : seedOptions(initialType),
  );

  // Stimulus subsection (per-question only — hidden when inside a group).
  const [stimulusOpen, setStimulusOpen] = useState(false);
  const [stimulusId, setStimulusId] = useState<string | null>(
    question?.stimulus?.id ?? null,
  );
  const [stimulusTitle, setStimulusTitle] = useState<string | null>(
    question?.stimulus?.title ?? null,
  );
  const [stimulusBody, setStimulusBody] = useState("");
  const [savingStimulus, setSavingStimulus] = useState(false);
  const [showStimulusLibrary, setShowStimulusLibrary] = useState(false);
  const [saveStimulusToLibrary, setSaveStimulusToLibrary] = useState(false);

  // Expanded form tab. When kisi-kisi is enabled, teachers switch between
  // Soal and Kisi-Kisi instead of scanning a long collapsed block below the
  // question editor. Default stays on Soal for authoring speed.
  const [activeTab, setActiveTab] = useState<"question" | "kisi">("question");

  // Phase 9.8 — inline kisi-kisi metadata. Pre-fill from the bound
  // slot when present (template clone or auto-blueprint), otherwise
  // start blank for the user to fill in.
  const slotMeta = slot ?? question?.slot ?? null;
  const [kkCP, setKkCP] = useState(slotMeta?.capaianPembelajaran ?? "");
  const [kkElemen, setKkElemen] = useState(slotMeta?.elemenCp ?? "");
  const [kkTP, setKkTP] = useState(slotMeta?.tujuanPembelajaran ?? "");
  const [kkMateriPokok, setKkMateriPokok] = useState(slotMeta?.materiPokok ?? "");
  const [kkKelas, setKkKelas] = useState(slotMeta?.kelas ?? "");
  const [kkSemester, setKkSemester] = useState(slotMeta?.semester ?? "");
  const [kkIndikatorSoal, setKkIndikatorSoal] = useState(slotMeta?.indikatorSoal ?? "");
  const [kkCognitive, setKkCognitive] = useState(
    slotMeta?.cognitiveLevel ?? "",
  );
  const [kkDifficulty, setKkDifficulty] = useState(slotMeta?.difficulty ?? "");
  const accordionRef = useRef<HTMLDivElement>(null);

  // Counter for the collapsible Kisi-Kisi header pill ("X/N"). The
  // total is dynamic per blueprint type — reguler counts the five
  // Kurikulum Merdeka fields (CP/Elemen CP/TP/Materi Pokok/Kelas/Semester/Indikator Soal/Cognitive/Difficulty). When the slot is locked
  // from a template the counter still reports filled state so users
  // see what's already attached.
  const { kkFilledCount, kkTotalCount } = useMemo(() => {
    const filledStr = (s: string) => s.trim().length > 0;
    const fields = [
      kkCP,
      kkElemen,
      kkTP,
      kkMateriPokok,
      kkKelas,
      kkSemester,
      kkIndikatorSoal,
      kkCognitive,
      kkDifficulty,
    ];
    return {
      kkFilledCount: fields.filter(filledStr).length,
      kkTotalCount: fields.length,
    };
  }, [
    kkCP,
    kkElemen,
    kkTP,
    kkMateriPokok,
    kkKelas,
    kkSemester,
    kkIndikatorSoal,
    kkCognitive,
    kkDifficulty,
  ]);

  // Reset local state when the underlying question identity shifts.
  useEffect(() => {
    setType(initialType);
    setContent(normalizeRichTextForEditor(question?.content));
    setExplanation(normalizeRichTextForEditor(question?.explanation));
    setCorrectAnswer(normalizeRichTextForEditor(question?.correctAnswer));
    setPoints(String(question?.points ?? slot?.points ?? 1));
    setScoringMode(question?.scoringMode ?? "correct_all");
    setOptions(
      question?.options && question.options.length > 0
        ? question.options
        : seedOptions(initialType),
    );
    setStimulusId(question?.stimulus?.id ?? null);
    setStimulusTitle(question?.stimulus?.title ?? null);
    setStimulusBody("");
    const meta = slot ?? question?.slot ?? null;
    setKkCP(meta?.capaianPembelajaran ?? "");
    setKkElemen(meta?.elemenCp ?? "");
    setKkTP(meta?.tujuanPembelajaran ?? "");
    setKkMateriPokok(meta?.materiPokok ?? "");
    setKkKelas(meta?.kelas ?? "");
    setKkSemester(meta?.semester ?? "");
    setKkIndikatorSoal(meta?.indikatorSoal ?? "");
    setKkCognitive(meta?.cognitiveLevel ?? "");
    setKkDifficulty(meta?.difficulty ?? "");
    setErrors({});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [question?.id, slot?.id]);

  // Re-seed options when type changes (new accordions or type swap).
  useEffect(() => {
    if (question && question.questionType === type) return;
    setOptions(seedOptions(type));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type]);

  // Esc collapses an open accordion.
  useEffect(() => {
    if (!isOpen) return;
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        if (isDraft && onCancelDraft) onCancelDraft();
        else onToggle();
      }
    }
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [isOpen, isDraft, onCancelDraft, onToggle]);

  function updateOption(i: number, patch: Partial<QuestionOption>) {
    setOptions((prev) => prev.map((o, idx) => (idx === i ? { ...o, ...patch } : o)));
  }

  function addOption() {
    setOptions((prev) => [...prev, { content: "", isCorrect: false }]);
  }

  function removeOption(i: number) {
    setOptions((prev) => prev.filter((_, idx) => idx !== i));
  }

  async function handleSaveStimulusSnapshot() {
    if (!stimulusId) return;
    if (!stimulusTitle?.trim() || !stimulusBody.replace(/<[^>]+>/g, "").trim()) {
      toast({
        tone: "error",
        title: "Lengkapi stimulus",
        description: "Stimulus butuh judul dan isi sebelum disimpan.",
      });
      return;
    }
    setSavingStimulus(true);
    const res = await updateStimulus(stimulusId, {
      title: stimulusTitle.trim(),
      content: stimulusBody,
    });
    if (res.error) {
      setSavingStimulus(false);
      toast({ tone: "error", title: "Gagal simpan stimulus", description: res.error.message });
      return;
    }
    if (saveStimulusToLibrary) {
      const promoted = await promoteStimulus(stimulusId);
      if (promoted.error && promoted.error.code !== "invalid_state") {
        setSavingStimulus(false);
        toast({ tone: "error", title: "Stimulus tersimpan, tetapi gagal masuk library", description: promoted.error.message });
        return;
      }
    }
    setSavingStimulus(false);
    setStimulusOpen(false);
    toast({ tone: "success", title: "Stimulus disimpan" });
  }

  async function handleDeleteStimulus() {
    if (!stimulusId) return;
    setSavingStimulus(true);
    const currentId = stimulusId;
    if (question) {
      const res = await updateQuestion(question.id, { stimulusId: "" });
      if (res.error) {
        setSavingStimulus(false);
        toast({ tone: "error", title: "Gagal melepas stimulus", description: res.error.message });
        return;
      }
    }
    const archived = await archiveStimulus(currentId);
    setSavingStimulus(false);
    if (archived.error) {
      toast({ tone: "error", title: "Stimulus dilepas, tetapi gagal diarsipkan", description: archived.error.message });
    } else {
      toast({ tone: "success", title: "Stimulus dihapus" });
    }
    setStimulusId(null);
    setStimulusTitle(null);
    setStimulusBody("");
    setStimulusOpen(false);
  }

  async function handleSelectStimulus(s: Stimulus) {
    setStimulusId(s.id);
    setStimulusTitle(s.title);
    setStimulusBody(normalizeRichTextForEditor(s.content));
    setSaveStimulusToLibrary(false);
    if (!question) return;
    setSavingStimulus(true);
    const res = await updateQuestion(question.id, { stimulusId: s.id });
    setSavingStimulus(false);
    if (res.error) {
      toast({ tone: "error", title: "Gagal mengikat stimulus", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Stimulus terikat ke soal" });
  }

  async function handleUseLibraryStimulus(s: Stimulus) {
    setShowStimulusLibrary(false);
    setStimulusId(s.id);
    setStimulusTitle(s.title);
    setStimulusBody(normalizeRichTextForEditor(s.content));
    setSaveStimulusToLibrary(false);
    if (!question) return;
    setSavingStimulus(true);
    const res = await updateQuestion(question.id, { stimulusId: s.id });
    setSavingStimulus(false);
    if (res.error) {
      toast({ tone: "error", title: "Gagal mengikat stimulus", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Stimulus dari library dimuat. Silakan edit lalu simpan jika perlu." });
  }

  async function loadStimulusSnapshot() {
    if (!stimulusId || stimulusBody) return;
    const res = await getStimulus(stimulusId);
    if (res.data) {
      setStimulusTitle(res.data.title);
      setStimulusBody(normalizeRichTextForEditor(res.data.content));
      setSaveStimulusToLibrary(false);
    }
  }

  async function handleOpenStimulusEditor() {
    setStimulusOpen((v) => !v);
    await loadStimulusSnapshot();
  }

  useEffect(() => {
    if (!stimulusId || isInsideGroup) return;
    void loadStimulusSnapshot();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stimulusId, isInsideGroup]);

  async function handleSave() {
    if (!content.trim()) {
      setErrors({ content: "Konten soal tidak boleh kosong" });
      return;
    }
    if (type === "multiple_choice") {
      const seen = new Map<string, number>();
      const duplicate = options.find((opt) => {
        const key = opt.content.trim().toLowerCase().replace(/\s+/g, " ");
        if (!key) return false;
        const count = seen.get(key) ?? 0;
        seen.set(key, count + 1);
        return count > 0;
      });
      if (duplicate) {
        setErrors({ options: "Opsi pilihan ganda tidak boleh sama/duplikat" });
        return;
      }
    }
    setErrors({});
    setSaving(true);

    const payload: CreateQuestionPayload = {
      sectionId: defaultSectionId || question?.sectionId || undefined,
      questionType: type,
      content,
      explanation: explanation || undefined,
      correctAnswer: correctAnswer || undefined,
      points: points ? Number(points) : 1,
      scoringMode,
      options:
        type === "multiple_choice" || type === "true_false"
          ? options
          : undefined,
    };

    // Stimulus axis: when inside a group, group_id wins (handled by parent
    // via defaultGroupId or pre-existing question.groupId). When solo,
    // the per-question stimulus_id is sent.
    if (isInsideGroup) {
      if (defaultGroupId) payload.groupId = defaultGroupId;
    } else {
      // Solo path — user may have set or cleared the stimulus. Empty
      // string clears server-side; undefined leaves untouched on update.
      if (question) {
        const initial = question.stimulus?.id ?? null;
        if (stimulusId !== initial) {
          payload.stimulusId = stimulusId ?? "";
        }
      } else if (stimulusId) {
        payload.stimulusId = stimulusId;
      }
    }

    // Phase 9.8 — inline kisi-kisi fields. Forward when KK is on.
    // Per user feedback (Phase 9.10): even template-bound slots stay
    // editable; edits write through to the slot row in the same tx,
    // and the change reflects on subsequent canvas reloads. The
    // visual lock indicator stays so users know the metadata source.
    if (usesKisiKisi) {
      payload.cognitiveLevel = kkCognitive;
      payload.difficulty = kkDifficulty;
      payload.capaianPembelajaran = kkCP;
      payload.elemenCp = kkElemen;
      payload.tujuanPembelajaran = kkTP;
      payload.materiPokok = kkMateriPokok;
      payload.kelas = kkKelas;
      payload.semester = kkSemester;
      payload.indikatorSoal = kkIndikatorSoal;

    }

    let res;
    if (question) {
      res = await updateQuestion(question.id, payload);
    } else if (slot) {
      // Slot-anchored creation. Server takes the slot's points / type
      // defaults if the payload doesn't override them.
      res = await createQuestionFromSlot(examId, slot.id, payload);
    } else {
      res = await createQuestion(examId, payload);
    }
    setSaving(false);

    if (res.error) {
      const fields = res.error.fields ?? {};
      if (Object.keys(fields).length > 0) {
        setErrors(fields);
        if (fields.capaianPembelajaran || fields.elemenCp || fields.tujuanPembelajaran || fields.materiPokok || fields.kelas || fields.semester || fields.indikatorSoal || fields.cognitiveLevel || fields.difficulty) {
          setActiveTab("kisi");
        }
      } else {
        toast({
          tone: "error",
          title: "Gagal menyimpan",
          description: res.error.message,
        });
      }
      return;
    }
    onSaved();
  }

  const typeLocked = !!slotMeta;

  return (
    <div
      ref={accordionRef}
      className={cn(
        // overflow-hidden was clipping the SelectField popover for
        // cognitive level / difficulty inside the kisi-kisi section.
        // Use overflow-visible when expanded so dropdowns can escape
        // the card; collapsed state keeps the rounded clip.
        isOpen ? "overflow-visible" : "overflow-hidden",
        "rounded-lg border bg-[var(--card)] transition-all",
        isOpen
          ? "border-[var(--brand)]/40 shadow-sm"
          : "border-[var(--border)] hover:border-[var(--border-strong)]",
      )}
    >
      {/* Collapsed / header row */}
      <div className="flex items-center gap-2 px-2.5 py-2">
        {dragHandleProps && canEdit && (
          <button
            type="button"
            aria-label="Drag untuk pindah"
            {...dragHandleProps}
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] cursor-grab active:cursor-grabbing transition-colors"
          >
            <GripVertical size={13} />
          </button>
        )}
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={isOpen}
          aria-label={isOpen ? "Collapse question" : "Expand question"}
          className="flex flex-1 items-center gap-2 min-w-0 text-left"
        >
          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-[var(--muted)] font-mono text-[10.5px] font-semibold text-[var(--muted-foreground)]">
            {index}
          </span>
          {isOpen ? (
            <ChevronDown
              size={13}
              className="shrink-0 text-[var(--muted-foreground)]"
            />
          ) : (
            <ChevronRight
              size={13}
              className="shrink-0 text-[var(--muted-foreground)]"
            />
          )}
          <div className="flex-1 min-w-0">
            <p className="truncate text-[12.5px] text-[var(--foreground)]">
              {question?.content ? (
                stripHtmlPreview(question.content, 140)
              ) : (
                <span className="italic text-[var(--muted-foreground)]">
                  {slot ? "Slot kosong — klik untuk tulis soal" : "Soal baru..."}
                </span>
              )}
            </p>
            <div className="mt-0.5 flex items-center gap-1.5 flex-wrap">
              <span className="rounded-md bg-[var(--muted)] px-1.5 py-0.5 text-[9.5px] font-medium text-[var(--muted-foreground)]">
                {QUESTION_TYPE_LABEL[type] ?? type}
              </span>
              <span className="text-[10px] text-[var(--muted-foreground)]">
                {Number(points || 0)} pt
              </span>
              {slotMeta?.elemenCp && (
                <span className="rounded-md bg-[var(--brand-soft)] px-1.5 py-0.5 text-[9.5px] font-medium text-[var(--brand)]">
                  {slotMeta.elemenCp}
                </span>
              )}
              {!isInsideGroup && stimulusTitle && (
                <span className="rounded-md bg-[var(--accent)] px-1.5 py-0.5 text-[9.5px] font-medium text-[var(--muted-foreground)] truncate max-w-[40%]">
                  Stim: {stimulusTitle}
                </span>
              )}
            </div>
          </div>
          {saving && (
            <Loader2
              size={12}
              className="shrink-0 animate-spin text-[var(--muted-foreground)]"
            />
          )}
        </button>
        {canEdit && question && !isDraft && (
          <InlineMagicPopover
            entityKind="question"
            entityId={question.id}
            examId={examId}
            className="shrink-0"
          />
        )}
        {canEdit && question && onDelete && !isOpen && (
          <button
            type="button"
            onClick={onDelete}
            aria-label="Hapus soal"
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--destructive-soft)] hover:text-[var(--destructive)] transition-colors"
          >
            <Trash2 size={12} />
          </button>
        )}
        {isDraft && canEdit && (
          <InlineMagicPopover
            entityKind="draft"
            entityId={
              defaultGroupId
                ? `group:${defaultGroupId}`
                : defaultSectionId
                ? `section:${defaultSectionId}`
                : ""
            }
            examId={examId}
            className="shrink-0"
          />
        )}
        {isDraft && onCancelDraft && (
          <button
            type="button"
            onClick={onCancelDraft}
            aria-label="Batalkan draft"
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
          >
            <X size={12} />
          </button>
        )}
      </div>

      {/* Expanded body */}
      {isOpen && (
        <div className="border-t border-[var(--brand)]/30 bg-[var(--background)] p-3 space-y-3 shadow-[inset_0_1px_0_var(--brand-soft)] ring-1 ring-inset ring-[var(--brand)]/10">
          {slotMeta && (
            <div className="rounded-md border border-[var(--brand)]/30 bg-[var(--brand-soft)]/60 px-2.5 py-1.5 text-[10.5px] text-[var(--brand)]">
              <span className="font-semibold">
                Slot kisi-kisi #{slotMeta.position + 1}
                {slotMeta.elemenCp ? `: ${slotMeta.elemenCp}` : ""}
              </span>
              {slotMeta.materiPokok ? ` — ${slotMeta.materiPokok}` : ""}
              {" · tipe terkunci, points default "}
              {slotMeta.points}
            </div>
          )}

          {usesKisiKisi && (
            <div className="flex rounded-lg border border-[var(--border)] bg-[var(--card)] p-0.5 shadow-sm">
              <button
                type="button"
                onClick={() => setActiveTab("question")}
                className={cn(
                  "flex-1 rounded-md px-2.5 py-1.5 text-[11px] font-semibold transition-all",
                  activeTab === "question"
                    ? "bg-[var(--primary)] text-[var(--primary-foreground)] shadow-sm"
                    : "text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]",
                )}
              >
                Soal
              </button>
              <button
                type="button"
                onClick={() => setActiveTab("kisi")}
                className={cn(
                  "flex-1 rounded-md px-2.5 py-1.5 text-[11px] font-semibold transition-all",
                  activeTab === "kisi"
                    ? "bg-[var(--primary)] text-[var(--primary-foreground)] shadow-sm"
                    : "text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]",
                )}
              >
                Kisi-Kisi <span className="ml-1 text-[10px] opacity-80">{kkFilledCount}/{kkTotalCount}</span>
              </button>
            </div>
          )}

          {(!usesKisiKisi || activeTab === "question") && <>
          {/* 1. Tipe Soal + Points (metadata row, points top-right) */}
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_120px]">
            <SelectField
              label="Tipe soal"
              value={type}
              onChange={(v) => setType(v as QuestionType)}
              options={QUESTION_TYPE_OPTIONS}
              disabled={typeLocked || !canEdit}
            />
            <InputField
              label="Poin"
              type="number"
              value={points}
              onChange={(e) => setPoints(e.target.value)}
              disabled={!canEdit}
            />
          </div>

          {/* 2. Stimulus subsection (per-question only) */}
          {!isInsideGroup && (
            <StimulusEditorPanel
              canEdit={canEdit}
              open={stimulusOpen}
              stimulusId={stimulusId}
              title={stimulusTitle}
              body={stimulusBody}
              saving={savingStimulus}
              saveToLibrary={saveStimulusToLibrary}
              showLibrary={showStimulusLibrary}
              onToggle={handleOpenStimulusEditor}
              onTitleChange={setStimulusTitle}
              onBodyChange={setStimulusBody}
              onSaveToLibraryChange={setSaveStimulusToLibrary}
              onSave={handleSaveStimulusSnapshot}
              onDelete={handleDeleteStimulus}
              onSelect={handleSelectStimulus}
              onClear={() => {
                setStimulusId(null);
                setStimulusTitle(null);
                setStimulusBody("");
              }}
              onOpenLibrary={() => setShowStimulusLibrary(true)}
              onCloseLibrary={() => setShowStimulusLibrary(false)}
              onSelectLibrary={handleUseLibraryStimulus}
            />
          )}

          {/* 3. Konten Soal — rich text editor with LaTeX */}
          <div>
            <label className="mb-1 block text-[11px] font-medium text-[var(--muted-foreground)]">
              Soal
            </label>
            <RichEditor
              value={content}
              onChange={setContent}
              minRows={3}
              placeholder="Tulis soal di sini. Gunakan toolbar untuk format atau LaTeX (\u03a3)."
              error={errors.content}
              disabled={!canEdit}
              ariaLabel="Konten soal"
            />
          </div>

          {/* 4. Opsi (only for MCQ / TF) */}
          {(type === "multiple_choice" || type === "true_false") && (
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <p className="text-[11px] font-semibold text-[var(--foreground)]">
                  Opsi ({options.length})
                </p>
                {type === "multiple_choice" && options.length < 10 && canEdit && (
                  <button
                    type="button"
                    onClick={addOption}
                    className="inline-flex items-center gap-1 text-[10.5px] font-medium text-[var(--brand)] hover:underline"
                  >
                    <Plus size={10} /> Tambah opsi
                  </button>
                )}
              </div>
              {options.map((opt, i) => (
                <div
                  key={i}
                  className="flex items-center gap-2 rounded-md border border-[var(--border)] bg-[var(--card)] px-2 py-1.5"
                >
                  <button
                    type="button"
                    aria-label={
                      opt.isCorrect ? "Tandai sebagai salah" : "Tandai sebagai benar"
                    }
                    onClick={() => updateOption(i, { isCorrect: !opt.isCorrect })}
                    disabled={!canEdit}
                    className={cn(
                      "flex h-6 w-6 shrink-0 items-center justify-center rounded-md transition-colors",
                      opt.isCorrect
                        ? "bg-[var(--success-soft)] text-[var(--success)]"
                        : "bg-[var(--muted)] text-[var(--muted-foreground)] hover:bg-[var(--border)]",
                      !canEdit && "opacity-60 cursor-not-allowed",
                    )}
                  >
                    {opt.isCorrect ? <Check size={11} /> : <Circle size={9} />}
                  </button>
                  <input
                    type="text"
                    value={opt.content}
                    onChange={(e) => updateOption(i, { content: e.target.value })}
                    disabled={type === "true_false" || !canEdit}
                    className="flex-1 bg-transparent text-[12px] text-[var(--foreground)] outline-none disabled:cursor-not-allowed disabled:opacity-60"
                    aria-label={`Opsi ${i + 1}`}
                  />
                  {type === "multiple_choice" && options.length > 2 && canEdit && (
                    <button
                      type="button"
                      onClick={() => removeOption(i)}
                      aria-label="Hapus opsi"
                      className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--destructive-soft)] hover:text-[var(--destructive)] transition-colors"
                    >
                      <Trash2 size={10} />
                    </button>
                  )}
                </div>
              ))}
              {errors.options && (
                <p className="text-[11px] font-medium text-[var(--danger)]">
                  {errors.options}
                </p>
              )}
              {type === "multiple_choice" && (
                <SelectField
                  label="Mode skoring"
                  value={scoringMode}
                  onChange={(v) => setScoringMode(v as ScoringMode)}
                  options={SCORING_MODE_OPTIONS}
                  disabled={!canEdit}
                />
              )}
            </div>
          )}

          {/* Reference / rubric (short answer + essay) sits between Opsi
              and Penjelasan since it's the analog of Opsi for those types. */}
          {(type === "short_answer" || type === "essay") && (
            <div>
              <label className="mb-1 block text-[11px] font-medium text-[var(--muted-foreground)]">
                {type === "short_answer"
                  ? "Reference answer (opsional)"
                  : "Rubrik / catatan grading (opsional)"}
              </label>
              <RichEditor
                value={correctAnswer}
                onChange={setCorrectAnswer}
                minRows={2}
                placeholder={type === "short_answer" ? "Jawaban rujukan." : "Rubrik atau catatan grading."}
                disabled={!canEdit}
                ariaLabel={type === "short_answer" ? "Reference answer" : "Rubrik"}
              />
            </div>
          )}

          {/* 5. Penjelasan — rich text editor */}
          <div>
            <label className="mb-1 block text-[11px] font-medium text-[var(--muted-foreground)]">
              Penjelasan (opsional)
            </label>
            <RichEditor
              value={explanation}
              onChange={setExplanation}
              minRows={2}
              placeholder="Penjelasan jawaban (ditampilkan setelah submit)."
              disabled={!canEdit}
              ariaLabel="Penjelasan"
            />
          </div>

          </>}

          {usesKisiKisi && activeTab === "kisi" && (
            <div className="rounded-xl border border-[var(--brand)]/30 bg-[var(--card)] p-3 shadow-sm">
              <div className="mb-3 flex items-start justify-between gap-3">
                <div>
                  <p className="text-[12px] font-semibold text-[var(--foreground)]">Metadata Kisi-Kisi</p>
                  <p className="text-[10.5px] text-[var(--muted-foreground)]">
                    Isi CP, Elemen CP, TP, materi pokok, kelas/semester, indikator soal, dan level kognitif.
                  </p>
                </div>
                <span className="rounded-full bg-[var(--brand-soft)] px-2 py-0.5 text-[10px] font-semibold text-[var(--brand)]">
                  {kkFilledCount}/{kkTotalCount}
                </span>
              </div>
              <MerdekaKisiKisiFields
                capaianPembelajaran={kkCP}
                elemenCp={kkElemen}
                tujuanPembelajaran={kkTP}
                materiPokok={kkMateriPokok}
                kelas={kkKelas}
                semester={kkSemester}
                cognitiveLevel={kkCognitive}
                difficulty={kkDifficulty}
                indikatorSoal={kkIndikatorSoal}
                onCapaianPembelajaran={setKkCP}
                onElemenCp={setKkElemen}
                onTujuanPembelajaran={setKkTP}
                onMateriPokok={setKkMateriPokok}
                onKelas={setKkKelas}
                onSemester={setKkSemester}
                onCognitiveLevel={setKkCognitive}
                onDifficulty={setKkDifficulty}
                onIndikatorSoal={setKkIndikatorSoal}
                errors={errors}
              />
            </div>
          )}

          {/* Footer */}
          <div className="flex items-center justify-end gap-2 pt-1">
            {question && onDelete && canEdit && (
              <button
                type="button"
                onClick={onDelete}
                disabled={saving}
                className="mr-auto inline-flex h-8 items-center gap-1 rounded-md text-[11.5px] font-medium text-[var(--destructive)] hover:bg-[var(--destructive-soft)] px-2 disabled:opacity-50 transition-colors"
              >
                <Trash2 size={12} /> Hapus
              </button>
            )}
            <button
              type="button"
              onClick={() => {
                if (isDraft && onCancelDraft) onCancelDraft();
                else onToggle();
              }}
              disabled={saving}
              className="h-8 rounded-md px-3 text-[11.5px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)] disabled:opacity-50 transition-colors"
            >
              Batal
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={saving || !canEdit}
              className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[var(--primary)] px-3 text-[11.5px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 disabled:opacity-50 transition-all"
            >
              {saving && <Loader2 size={11} className="animate-spin" />}
              {question ? "Simpan" : <><Sparkles size={11} /> Simpan soal</>}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function seedOptions(t: QuestionType): QuestionOption[] {
  if (t === "multiple_choice") {
    return [
      { content: "", isCorrect: false },
      { content: "", isCorrect: false },
      { content: "", isCorrect: false },
      { content: "", isCorrect: false },
    ];
  }
  if (t === "true_false") {
    return [
      { content: "True", isCorrect: true },
      { content: "False", isCorrect: false },
    ];
  }
  return [];
}
