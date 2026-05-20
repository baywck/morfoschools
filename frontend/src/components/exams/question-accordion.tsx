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
  createQuestion,
  createQuestionFromSlot,
  updateQuestion,
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
import { KisiKisiFields } from "@/components/blueprint/kisi-kisi-fields";
import { RichEditor } from "@/components/ui/rich-editor";
import { stripHtmlPreview } from "@/components/ui/rendered-content";
import { StimulusPicker } from "@/components/exams/stimulus-picker";
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
  /** Phase 9.8: render the inline Kisi-Kisi subsection (KD / materi /
   *  indikator / cognitive level / difficulty or AKM dimensions). The
   *  subsection writes back to the bound slot when present, or rides
   *  along on the create payload so the backend can mint a slot. */
  usesKisiKisi?: boolean;
  /** True when blueprint_type is AKM literasi/numerasi. Switches the
   *  inline kisi-kisi layout to AKM-specific labels (Konten / Konteks
   *  / Proses Kognitif + Level 1–5). */
  isAkm?: boolean;
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
  isAkm = false,
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
  const [content, setContent] = useState(question?.content ?? "");
  const [explanation, setExplanation] = useState(question?.explanation ?? "");
  const [correctAnswer, setCorrectAnswer] = useState(
    question?.correctAnswer ?? "",
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

  // Kisi-Kisi subsection collapse state. Default collapsed so the
  // accordion stays focused on content authoring; teachers expand
  // when they want to attach pedagogical metadata.
  const [kkOpen, setKkOpen] = useState(false);

  // Phase 9.8 — inline kisi-kisi metadata. Pre-fill from the bound
  // slot when present (template clone or auto-blueprint), otherwise
  // start blank for the user to fill in.
  const slotMeta = slot ?? question?.slot ?? null;
  const [kkCompetency, setKkCompetency] = useState(
    slotMeta?.competencyCode ?? "",
  );
  const [kkCompetencyDescription, setKkCompetencyDescription] = useState(
    slotMeta?.competencyDescription ?? "",
  );
  const [kkMateri, setKkMateri] = useState(slotMeta?.materi ?? "");
  const [kkIndikator, setKkIndikator] = useState(slotMeta?.indikator ?? "");
  const [kkCognitive, setKkCognitive] = useState(
    slotMeta?.cognitiveLevel ?? "",
  );
  const [kkDifficulty, setKkDifficulty] = useState(slotMeta?.difficulty ?? "");
  const [kkAkmKonten, setKkAkmKonten] = useState(slotMeta?.akmKonten ?? "");
  const [kkAkmKonteks, setKkAkmKonteks] = useState(slotMeta?.akmKonteks ?? "");
  const [kkAkmProses, setKkAkmProses] = useState(slotMeta?.akmProses ?? "");
  const [kkAkmLevel, setKkAkmLevel] = useState(
    slotMeta?.akmLevel != null ? String(slotMeta.akmLevel) : "",
  );

  const accordionRef = useRef<HTMLDivElement>(null);

  // Counter for the collapsible Kisi-Kisi header pill ("X/N"). The
  // total is dynamic per blueprint type — reguler counts the five
  // pedagogical fields (KD/Materi/Indikator/Cognitive/Difficulty),
  // AKM swaps Cognitive+Difficulty for Konten/Konteks/Proses/Level
  // (eight fields with KD/Materi/Indikator). When the slot is locked
  // from a template the counter still reports filled state so users
  // see what's already attached.
  const { kkFilledCount, kkTotalCount } = useMemo(() => {
    const filledStr = (s: string) => s.trim().length > 0;
    if (isAkm) {
      const fields = [
        kkCompetency,
        kkMateri,
        kkIndikator,
        kkAkmKonten,
        kkAkmKonteks,
        kkAkmProses,
        kkAkmLevel,
      ];
      return {
        kkFilledCount: fields.filter(filledStr).length,
        kkTotalCount: fields.length,
      };
    }
    const fields = [
      kkCompetency,
      kkMateri,
      kkIndikator,
      kkCognitive,
      kkDifficulty,
    ];
    return {
      kkFilledCount: fields.filter(filledStr).length,
      kkTotalCount: fields.length,
    };
  }, [
    isAkm,
    kkCompetency,
    kkMateri,
    kkIndikator,
    kkCognitive,
    kkDifficulty,
    kkAkmKonten,
    kkAkmKonteks,
    kkAkmProses,
    kkAkmLevel,
  ]);

  // Reset local state when the underlying question identity shifts.
  useEffect(() => {
    setType(initialType);
    setContent(question?.content ?? "");
    setExplanation(question?.explanation ?? "");
    setCorrectAnswer(question?.correctAnswer ?? "");
    setPoints(String(question?.points ?? slot?.points ?? 1));
    setScoringMode(question?.scoringMode ?? "correct_all");
    setOptions(
      question?.options && question.options.length > 0
        ? question.options
        : seedOptions(initialType),
    );
    setStimulusId(question?.stimulus?.id ?? null);
    setStimulusTitle(question?.stimulus?.title ?? null);
    const meta = slot ?? question?.slot ?? null;
    setKkCompetency(meta?.competencyCode ?? "");
    setKkCompetencyDescription(meta?.competencyDescription ?? "");
    setKkMateri(meta?.materi ?? "");
    setKkIndikator(meta?.indikator ?? "");
    setKkCognitive(meta?.cognitiveLevel ?? "");
    setKkDifficulty(meta?.difficulty ?? "");
    setKkAkmKonten(meta?.akmKonten ?? "");
    setKkAkmKonteks(meta?.akmKonteks ?? "");
    setKkAkmProses(meta?.akmProses ?? "");
    setKkAkmLevel(meta?.akmLevel != null ? String(meta.akmLevel) : "");
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

  async function handleSave() {
    if (!content.trim()) {
      setErrors({ content: "Konten soal tidak boleh kosong" });
      return;
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
      if (isAkm) {
        payload.akmKonten = kkAkmKonten;
        payload.akmKonteks = kkAkmKonteks;
        payload.akmProses = kkAkmProses;
        if (kkAkmLevel) {
          const lvl = Number(kkAkmLevel);
          if (!Number.isNaN(lvl)) payload.akmLevel = lvl;
        }
      } else {
        payload.cognitiveLevel = kkCognitive;
        payload.difficulty = kkDifficulty;
      }
      payload.competencyCode = kkCompetency;
      payload.competencyDescription = kkCompetencyDescription;
      payload.materi = kkMateri;
      payload.indikator = kkIndikator;
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
              {slotMeta?.competencyCode && (
                <span className="rounded-md bg-[var(--brand-soft)] px-1.5 py-0.5 text-[9.5px] font-medium text-[var(--brand)]">
                  {slotMeta.competencyCode}
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
        <div className="border-t border-[var(--border)] bg-[var(--accent)]/30 p-3 space-y-3">
          {slotMeta && (
            <div className="rounded-md border border-[var(--brand)]/30 bg-[var(--brand-soft)]/60 px-2.5 py-1.5 text-[10.5px] text-[var(--brand)]">
              <span className="font-semibold">
                Slot kisi-kisi #{slotMeta.position + 1}
                {slotMeta.competencyCode ? `: ${slotMeta.competencyCode}` : ""}
              </span>
              {slotMeta.materi ? ` — ${slotMeta.materi}` : ""}
              {" · tipe terkunci, points default "}
              {slotMeta.points}
            </div>
          )}

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
            <div className="rounded-lg border border-dashed border-[var(--border-strong)] bg-[var(--background)]">
              <button
                type="button"
                onClick={() => setStimulusOpen((v) => !v)}
                className="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors hover:bg-[var(--muted)]/40"
              >
                {stimulusOpen ? (
                  <ChevronDown size={12} className="text-[var(--muted-foreground)]" />
                ) : (
                  <ChevronRight size={12} className="text-[var(--muted-foreground)]" />
                )}
                <span className="text-[11.5px] font-semibold text-[var(--foreground)]">
                  Stimulus / Bacaan
                </span>
                {stimulusTitle ? (
                  <span className="text-[10.5px] text-[var(--muted-foreground)] truncate">
                    · {stimulusTitle}
                  </span>
                ) : (
                  <span className="text-[10.5px] italic text-[var(--muted-foreground)]">
                    + Tambah stimulus
                  </span>
                )}
              </button>
              {stimulusOpen && (
                <div className="px-3 pb-3">
                  <StimulusPicker
                    value={stimulusId}
                    onSelect={(s: Stimulus) => {
                      setStimulusId(s.id);
                      setStimulusTitle(s.title);
                    }}
                    onClear={() => {
                      setStimulusId(null);
                      setStimulusTitle(null);
                    }}
                  />
                </div>
              )}
            </div>
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
              <textarea
                value={correctAnswer}
                onChange={(e) => setCorrectAnswer(e.target.value)}
                rows={2}
                disabled={!canEdit}
                className="w-full rounded-lg border border-[var(--border)] bg-[var(--card)] px-3 py-2 text-[13px] text-[var(--foreground)] outline-none focus:border-[var(--brand)] focus:ring-2 focus:ring-[var(--field-ring)]"
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

          {/* 6. Kisi-Kisi subsection — collapsible (default closed) */}
          {usesKisiKisi && (
            <KisiKisiSection
              isOpen={kkOpen}
              onToggle={() => setKkOpen((v) => !v)}
              filledCount={kkFilledCount}
              totalCount={kkTotalCount}
              locked={false}
              isAkm={isAkm}
              canEdit={canEdit}
              competency={kkCompetency}
              competencyDescription={kkCompetencyDescription}
              materi={kkMateri}
              indikator={kkIndikator}
              cognitive={kkCognitive}
              difficulty={kkDifficulty}
              akmKonten={kkAkmKonten}
              akmKonteks={kkAkmKonteks}
              akmProses={kkAkmProses}
              akmLevel={kkAkmLevel}
              onCompetency={setKkCompetency}
              onCompetencyDescription={setKkCompetencyDescription}
              onMateri={setKkMateri}
              onIndikator={setKkIndikator}
              onCognitive={setKkCognitive}
              onDifficulty={setKkDifficulty}
              onAkmKonten={setKkAkmKonten}
              onAkmKonteks={setKkAkmKonteks}
              onAkmProses={setKkAkmProses}
              onAkmLevel={setKkAkmLevel}
            />
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

// ──────────────────────────────────────────────────────────────────
// KisiKisiInline (Phase 9.8) — inline pedagogical metadata block.
// Two layouts: reguler (KD/Materi/Indikator + Cognitive + Difficulty)
// and AKM (KD/Materi/Indikator + Konten/Konteks/Proses + Level 1–5).
// ──────────────────────────────────────────────────────────────────

// Cognitive/difficulty/AKM-level option tables now live in the shared
// KisiKisiFields module so both the blueprint detail page slot form
// and the question accordion stay in sync.

function KisiKisiInline(props: {
  isAkm: boolean;
  locked: boolean;
  canEdit: boolean;
  competency: string;
  competencyDescription: string;
  materi: string;
  indikator: string;
  cognitive: string;
  difficulty: string;
  akmKonten: string;
  akmKonteks: string;
  akmProses: string;
  akmLevel: string;
  onCompetency: (v: string) => void;
  onCompetencyDescription: (v: string) => void;
  onMateri: (v: string) => void;
  onIndikator: (v: string) => void;
  onCognitive: (v: string) => void;
  onDifficulty: (v: string) => void;
  onAkmKonten: (v: string) => void;
  onAkmKonteks: (v: string) => void;
  onAkmProses: (v: string) => void;
  onAkmLevel: (v: string) => void;
}) {
  const readOnly = props.locked || !props.canEdit;
  // Delegated to the shared KisiKisiFields component so the blueprint
  // detail page slot sheet and the question accordion render the same
  // form. (Phase 9.10 unification.)
  return (
    <div className="p-3">
      <KisiKisiFields
        isAkm={props.isAkm}
        readOnly={readOnly}
        competency={props.competency}
        competencyDescription={props.competencyDescription}
        materi={props.materi}
        indikator={props.indikator}
        cognitive={props.cognitive}
        difficulty={props.difficulty}
        akmKonten={props.akmKonten}
        akmKonteks={props.akmKonteks}
        akmProses={props.akmProses}
        akmLevel={props.akmLevel}
        onCompetency={props.onCompetency}
        onCompetencyDescription={props.onCompetencyDescription}
        onMateri={props.onMateri}
        onIndikator={props.onIndikator}
        onCognitive={props.onCognitive}
        onDifficulty={props.onDifficulty}
        onAkmKonten={props.onAkmKonten}
        onAkmKonteks={props.onAkmKonteks}
        onAkmProses={props.onAkmProses}
        onAkmLevel={props.onAkmLevel}
      />
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────
// KisiKisiSection (Phase 9.9) — collapsible chrome around
// KisiKisiInline. Header carries the ClipboardList icon, a chevron
// that rotates on toggle, and a counter pill "X/N" showing how many
// fields the user has filled. Default state is collapsed so the
// authoring focus stays on content; teachers expand when they want to
// attach pedagogical metadata.
// ─────────────────────────────────────────────────────────────────

function KisiKisiSection(props: {
  isOpen: boolean;
  onToggle: () => void;
  filledCount: number;
  totalCount: number;
  locked: boolean;
  isAkm: boolean;
  canEdit: boolean;
  competency: string;
  competencyDescription: string;
  materi: string;
  indikator: string;
  cognitive: string;
  difficulty: string;
  akmKonten: string;
  akmKonteks: string;
  akmProses: string;
  akmLevel: string;
  onCompetency: (v: string) => void;
  onCompetencyDescription: (v: string) => void;
  onMateri: (v: string) => void;
  onIndikator: (v: string) => void;
  onCognitive: (v: string) => void;
  onDifficulty: (v: string) => void;
  onAkmKonten: (v: string) => void;
  onAkmKonteks: (v: string) => void;
  onAkmProses: (v: string) => void;
  onAkmLevel: (v: string) => void;
}) {
  const {
    isOpen,
    onToggle,
    filledCount,
    totalCount,
    locked,
    isAkm,
    canEdit,
    ...inlineProps
  } = props;

  return (
    <div className="rounded-lg border border-[var(--brand)]/30 bg-[var(--brand-soft)]/10">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={isOpen}
        aria-controls="kk-section-body"
        title={locked ? "Locked from template" : undefined}
        className="flex w-full items-center gap-2 rounded-t-lg px-3 py-2 text-left transition-colors hover:bg-[var(--brand-soft)]/30"
      >
        <ChevronDown
          size={13}
          className={cn(
            "shrink-0 text-[var(--brand)] transition-transform duration-200",
            isOpen ? "rotate-0" : "-rotate-90",
          )}
        />
        <ClipboardList size={13} className="shrink-0 text-[var(--brand)]" />
        <span className="text-[11.5px] font-semibold text-[var(--brand)]">
          Kisi-Kisi
        </span>
        {isAkm && (
          <span className="rounded-md bg-[var(--brand)] px-1.5 py-0.5 text-[9px] font-semibold uppercase text-white">
            AKM
          </span>
        )}
        <span className="ml-auto inline-flex items-center gap-1">
          {locked && (
            <span
              className="inline-flex items-center gap-1 text-[10px] text-[var(--muted-foreground)]"
              title="Locked from template"
            >
              <Lock size={10} /> Terkunci
            </span>
          )}
          <span
            className={cn(
              "rounded-full px-2 py-0.5 text-[10px] font-semibold tabular-nums",
              filledCount === totalCount && totalCount > 0
                ? "bg-[var(--success-soft)] text-[var(--success)]"
                : "bg-[var(--brand-soft)] text-[var(--brand)]",
            )}
          >
            {filledCount}/{totalCount}
          </span>
        </span>
      </button>
      {isOpen && (
        <div
          id="kk-section-body"
          className="border-t border-[var(--brand)]/20 bg-[var(--card)]"
        >
          <KisiKisiInline
            isAkm={isAkm}
            locked={locked}
            canEdit={canEdit}
            {...inlineProps}
          />
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
