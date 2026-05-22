"use client";

/**
 * InlineMagicPopover — hover-revealed ✨ button on entity cards
 * (questions, groups, blueprint slots) that opens a contextual
 * command menu for AI actions. Reuses the panel chat backend
 * (POST /api/v1/ai/chat) so proposals follow the existing two-phase
 * confirm flow: AI returns a proposal → harness shows it in the
 * sidebar AI panel → user confirms there.
 *
 * Usage:
 *   <InlineMagicPopover
 *     entityKind="question" | "group" | "slot"
 *     entityId={id}
 *     examId={examId}              // for question/group
 *     templateId={templateId}      // for slot
 *     onProposal={(content, proposal) => openAiPanelWith(content, proposal)}
 *   />
 *
 * Constraint: never mutates DB directly. Always goes through the
 * existing AI proposal flow so RBAC + duplicate detection + audit
 * events all apply identically.
 */

import { useState, useRef, useEffect } from "react";
import { Sparkles, Loader2, X, Wand2 } from "lucide-react";
import { cn } from "@/lib/cn";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

type EntityKind = "question" | "group" | "slot";

interface Command {
  /** Display label in the command menu */
  label: string;
  /** Short descriptor under the label */
  hint: string;
  /** Pre-composed user message sent to /api/v1/ai/chat */
  prompt: string;
  /** Free-text follow-up needed (e.g. user describes variant) */
  needsInput?: boolean;
  /** Placeholder for the input field when needsInput=true */
  inputHint?: string;
}

const QUESTION_COMMANDS: Command[] = [
  {
    label: "Perbaiki redaksi",
    hint: "Tata bahasa + kejelasan, tipe & jawaban tetap",
    prompt: "Tolong perbaiki redaksi soal ini agar lebih jelas dan tata bahasa benar. Pertahankan tipe soal, opsi, dan jawaban benar. Hasil: update_question dengan content yang sudah diperbaiki.",
  },
  {
    label: "Buat varian",
    hint: "Soal serupa dengan angka/konteks beda",
    prompt: "Buat 1 soal varian dari soal ini — tipe sama, level kognitif sama, tapi angka/konteks/skenario berbeda. Tambahkan ke section yang sama (atau group yang sama jika ada). Pakai create_question.",
  },
  {
    label: "Naikkan ke HOTS",
    hint: "Convert ke level kognitif lebih tinggi",
    prompt: "Tulis ulang soal ini ke level kognitif HOTS (analisis/evaluasi/mencipta). Pertahankan topik, tapi minta penalaran lebih tinggi. Pakai update_question.",
  },
  {
    label: "Tambah penjelasan",
    hint: "Generate explanation untuk jawaban benar",
    prompt: "Tulis penjelasan/pembahasan untuk jawaban benar soal ini. Pakai update_question dengan field explanation.",
  },
  {
    label: "Tambah opsi distractor",
    hint: "Generate opsi pengecoh tambahan (MCQ saja)",
    prompt: "Tambahkan opsi distractor (pengecoh) yang masuk akal untuk soal ini. Pakai update_question dengan options yang lebih banyak dan plausible. Hanya untuk multiple_choice.",
  },
  {
    label: "Convert tipe",
    hint: "MCQ ↔ essay ↔ true/false",
    prompt: "",
    needsInput: true,
    inputHint: "Convert ke tipe apa? (essay/multiple_choice/true_false/short_answer)",
  },
  {
    label: "Custom…",
    hint: "Tulis instruksi bebas",
    prompt: "",
    needsInput: true,
    inputHint: "Apa yang ingin diubah dari soal ini?",
  },
];

const GROUP_COMMANDS: Command[] = [
  {
    label: "Tambah soal ke group",
    hint: "Generate N soal lagi yang merujuk stimulus",
    prompt: "",
    needsInput: true,
    inputHint: "Berapa soal dan tipe apa? (mis: 3 soal multiple_choice)",
  },
  {
    label: "Refine stimulus",
    hint: "Perbaiki teks stimulus tanpa rusak soal",
    prompt: "Perbaiki redaksi stimulus group ini agar lebih jelas dan tata bahasa rapi. Pertahankan fakta utama. Pakai update_question_group dengan bodySnapshot/titleSnapshot baru.",
  },
  {
    label: "Generate distractor batch",
    hint: "Tambah pengecoh ke semua soal MCQ di group",
    prompt: "Untuk setiap soal multiple_choice di group ini yang masih punya <4 opsi, tambahkan opsi distractor plausible. Pakai update_question per soal.",
  },
  {
    label: "Custom…",
    hint: "Tulis instruksi bebas",
    prompt: "",
    needsInput: true,
    inputHint: "Apa yang ingin dilakukan terhadap group ini?",
  },
];

const SLOT_COMMANDS: Command[] = [
  {
    label: "Generate soal dari kisi-kisi",
    hint: "Buat soal sesuai KD/Materi/Indikator slot ini",
    prompt: "Buat 1 soal yang sesuai dengan kisi-kisi slot ini (KD, materi, indikator, cognitive level, difficulty). Pakai create_question dengan blueprintSlotId pointing ke slot ini.",
  },
  {
    label: "Custom…",
    hint: "Tulis instruksi bebas",
    prompt: "",
    needsInput: true,
    inputHint: "Apa yang ingin dilakukan dengan slot ini?",
  },
];

const COMMANDS_BY_KIND: Record<EntityKind, Command[]> = {
  question: QUESTION_COMMANDS,
  group: GROUP_COMMANDS,
  slot: SLOT_COMMANDS,
};

export interface InlineMagicPopoverProps {
  entityKind: EntityKind;
  entityId: string;
  examId?: string;
  templateId?: string;
  className?: string;
}

export function InlineMagicPopover({
  entityKind,
  entityId,
  examId,
  templateId,
  className,
}: InlineMagicPopoverProps) {
  const [open, setOpen] = useState(false);
  const [selectedCmd, setSelectedCmd] = useState<Command | null>(null);
  const [inputValue, setInputValue] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const popoverRef = useRef<HTMLDivElement | null>(null);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    function onClick(e: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        closeAll();
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") closeAll();
    }
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  function closeAll() {
    setOpen(false);
    setSelectedCmd(null);
    setInputValue("");
    setError(null);
  }

  async function dispatch(prompt: string) {
    setSending(true);
    setError(null);
    try {
      const csrfMatch = document.cookie.match(/csrf_token=([^;]+)/);
      const csrfToken = csrfMatch ? csrfMatch[1] : "";

      // Activate the AI panel so the user sees the proposal land.
      // The panel listens on the same scope key (exam:<id>) so the
      // session this turn appends to is the same one the panel shows.
      window.dispatchEvent(new CustomEvent("morfoschools:open-ai-panel"));

      const activeEntities: Record<string, string> = {};
      if (examId) activeEntities.examId = examId;
      if (templateId) activeEntities.templateId = templateId;
      if (entityKind === "question") activeEntities.questionId = entityId;
      else if (entityKind === "group") activeEntities.groupId = entityId;
      else if (entityKind === "slot") activeEntities.slotId = entityId;

      const res = await fetch(`${API_BASE}/api/v1/ai/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
        credentials: "include",
        body: JSON.stringify({
          message: prompt,
          shadow: { activeEntities },
        }),
      });
      const data = await res.json().catch(() => null);
      if (!res.ok) throw new Error(data?.error?.message || "Gagal kirim ke AI");

      // Notify the panel a new turn happened so it can refresh its
      // session display. The panel handles its own message rendering.
      window.dispatchEvent(
        new CustomEvent("morfoschools:ai-turn-completed", { detail: data }),
      );

      closeAll();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSending(false);
    }
  }

  function handleCommandClick(cmd: Command) {
    if (cmd.needsInput) {
      setSelectedCmd(cmd);
      setInputValue("");
      return;
    }
    void dispatch(cmd.prompt);
  }

  function handleInputSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedCmd) return;
    const trimmed = inputValue.trim();
    if (!trimmed) return;
    const finalPrompt = selectedCmd.prompt
      ? `${selectedCmd.prompt}\n\nDetail dari user: ${trimmed}`
      : trimmed;
    void dispatch(finalPrompt);
  }

  const commands = COMMANDS_BY_KIND[entityKind];

  return (
    <div className={cn("relative inline-block", className)} ref={popoverRef}>
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          setOpen((v) => !v);
        }}
        title="AI assist"
        aria-label="Buka menu AI"
        className={cn(
          "inline-flex h-7 w-7 items-center justify-center rounded-md transition-colors",
          "text-[var(--muted-foreground)] hover:text-[var(--brand)] hover:bg-[var(--brand-soft)]/40",
          open && "bg-[var(--brand-soft)] text-[var(--brand)]",
        )}
      >
        <Sparkles size={13} />
      </button>

      {open && (
        <div
          className="absolute right-0 z-50 mt-1.5 w-72 overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)] shadow-lg"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-center gap-2 border-b border-[var(--border)] bg-[var(--accent)]/30 px-3 py-2">
            <Wand2 className="h-3.5 w-3.5 text-[var(--brand)]" />
            <h4 className="flex-1 text-[11.5px] font-semibold text-[var(--foreground)]">
              AI assist {entityKind === "question" ? "soal" : entityKind === "group" ? "group" : "kisi-kisi"}
            </h4>
            <button
              type="button"
              onClick={closeAll}
              className="text-[var(--muted-foreground)] hover:text-[var(--foreground)]"
              aria-label="Tutup"
            >
              <X size={12} />
            </button>
          </div>

          {/* Command menu */}
          {!selectedCmd && (
            <div className="max-h-72 overflow-y-auto py-1">
              {commands.map((c) => (
                <button
                  key={c.label}
                  type="button"
                  onClick={() => handleCommandClick(c)}
                  disabled={sending}
                  className={cn(
                    "block w-full px-3 py-2 text-left transition-colors hover:bg-[var(--accent)]/40 disabled:opacity-50",
                  )}
                >
                  <p className="text-[12px] font-medium text-[var(--foreground)]">{c.label}</p>
                  <p className="text-[10.5px] text-[var(--muted-foreground)]">{c.hint}</p>
                </button>
              ))}
            </div>
          )}

          {/* Input form when command needs detail */}
          {selectedCmd && (
            <form onSubmit={handleInputSubmit} className="space-y-2 px-3 py-3">
              <p className="text-[11px] font-medium text-[var(--foreground)]">{selectedCmd.label}</p>
              <input
                type="text"
                autoFocus
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                placeholder={selectedCmd.inputHint}
                className="h-9 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-2.5 text-[12px] text-[var(--foreground)] outline-none focus:border-[var(--brand)] focus:ring-2 focus:ring-[var(--field-ring)]"
              />
              <div className="flex items-center justify-end gap-1.5">
                <button
                  type="button"
                  onClick={() => setSelectedCmd(null)}
                  className="h-7 rounded-md px-2.5 text-[11px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)]"
                >
                  Kembali
                </button>
                <button
                  type="submit"
                  disabled={sending || !inputValue.trim()}
                  className="inline-flex h-7 items-center gap-1.5 rounded-md bg-[var(--brand)] px-3 text-[11px] font-semibold text-white hover:opacity-90 disabled:opacity-50"
                >
                  {sending && <Loader2 size={10} className="animate-spin" />}
                  Kirim ke AI
                </button>
              </div>
            </form>
          )}

          {error && (
            <div className="border-t border-[var(--destructive)]/30 bg-[var(--destructive-soft)]/40 px-3 py-2 text-[10.5px] text-[var(--destructive)]">
              {error}
            </div>
          )}

          <div className="border-t border-[var(--border)] bg-[var(--accent)]/20 px-3 py-1.5 text-[9.5px] text-[var(--muted-foreground)]">
            Hasil akan muncul sebagai proposal di panel AI. Konfirmasi dengan &ldquo;ya&rdquo; di chat.
          </div>
        </div>
      )}
    </div>
  );
}
