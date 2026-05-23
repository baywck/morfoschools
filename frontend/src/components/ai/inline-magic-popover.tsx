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
import { createPortal } from "react-dom";
import { Sparkles, Loader2, X, Wand2 } from "lucide-react";
import { cn } from "@/lib/cn";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

type EntityKind = "question" | "group" | "slot" | "draft" | "exam";

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

const QUESTION_QUALITY_CONTRACT = `Kualitas soal wajib tinggi. Sebelum membuat soal, buat kisi-kisi INTERNAL dulu untuk mengontrol kualitas (topik, jenjang/asumsi, kompetensi, indikator konseptual, cognitiveLevel, difficulty), tetapi JANGAN tampilkan kisi-kisi internal kecuali user meminta. Jika exam memakai kisi-kisi dan soal baru belum punya blueprintSlotId, setelah soal dibuat lanjutkan dengan apply_question_kisi_kisi/bulk_apply_question_kisi_kisi untuk menyimpan kisi-kisi yang sesuai dan link ke soal. Jika data user kurang lengkap, gunakan asumsi wajar dan tuliskan asumsi singkat di awal. Setiap soal minimal memiliki stimulus/konteks; stimulus boleh berdiri sendiri atau langsung tertanam di stem soal. Untuk soal biasa, stem/content harus berupa konteks atau skenario 2-4 kalimat sebelum pertanyaan, bukan satu kalimat pendek seperti '... adalah untuk'. Jangan membuat stem terlalu pendek tanpa konteks kecuali user eksplisit meminta. Default tipe soal adalah multiple_choice; jangan membuat short_answer kecuali user eksplisit meminta. Setiap soal harus: sesuai topik dan jenjang; level kognitif jelas; tidak ambigu; tidak sekadar hafalan jika diminta level tinggi; opsi jawaban homogen, setara panjang/jenisnya, dan plausible; hanya satu jawaban benar; menyertakan kunci dan pembahasan singkat. Indikator internal harus konseptual, tidak menyalin redaksi soal, tidak membocorkan jawaban.`;

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
    label: "Extract kisi-kisi",
    hint: "Generate KD/Materi/Indikator dari soal ini",
    prompt: "Analisis soal fokus ini lalu rumuskan kisi-kisi lengkap. Bedakan field: competencyCode = kode KD saja (mis. 3.6); materi = topik/ruang lingkup materi tanpa awalan KD/kode; competencyDescription = uraian kompetensi yang diukur, bukan salinan materi; indikator = perilaku terukur yang konseptual. Indikator harus lebih umum dari redaksi soal, tidak menyalin frasa soal, tidak menyebut opsi/jawaban benar, dan tidak menjadi bocoran; tulis kompetensi yang diuji guru, bukan petunjuk untuk siswa. PAKAI apply_question_kisi_kisi dengan competencyCode, competencyDescription, materi, indikator, cognitiveLevel C1-C6, difficulty. Jangan pakai apply_blueprint_analysis / convert_questions_to_kisi_kisi.",
  },
  {
    label: "Generate dari kisi-kisi slot",
    hint: "Buat soal baru sesuai slot yang sudah terikat",
    prompt: "Soal ini sudah terikat ke blueprint slot. Generate 1 soal varian baru yang sesuai dengan kisi-kisi slot tersebut (KD/Materi/Indikator/cognitive level/difficulty sama), tapi dengan konteks/angka/skenario berbeda. Pakai create_question dengan blueprintSlotId yang sama.",
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
    label: "Buat / ganti stimulus",
    hint: "Generate teks stimulus untuk group ini",
    prompt: "",
    needsInput: true,
    inputHint: "Topik stimulus? (mis: dampak perubahan iklim di Indonesia)",
  },
  {
    label: "Tambah 1 soal sesuai stimulus",
    hint: "1 soal baru yang merujuk stimulus group",
    prompt: "",
    needsInput: true,
    inputHint: "Tipe + fokus soal? (mis: multiple_choice, level analisis HOTS)",
  },
  {
    label: "Tambah N soal ke group",
    hint: "Batch generate yang merujuk stimulus",
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
    label: "Generate kisi-kisi grup",
    hint: "Extract kisi-kisi dari semua soal di group",
    prompt: "Untuk setiap soal di group fokus, rumuskan kisi-kisi lengkap. Indikator harus konseptual: lebih umum dari redaksi soal, tidak menyalin frasa soal, tidak menyebut opsi/jawaban benar, dan tidak menjadi bocoran; tulis kompetensi yang diuji guru, bukan petunjuk untuk siswa. PAKAI bulk_apply_question_kisi_kisi SEKALI dengan items untuk semua questionId di group, replace=false. JANGAN pakai apply_blueprint_analysis / convert_questions_to_kisi_kisi.",
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
    label: "Generate N varian",
    hint: "Multiple soal serupa dengan kisi-kisi yang sama",
    prompt: "",
    needsInput: true,
    inputHint: "Berapa varian soal? (mis: 3)",
  },
  {
    label: "Refine kisi-kisi",
    hint: "Perbaiki redaksi indikator tanpa ubah cognitive level",
    prompt: "Perbaiki redaksi indikator slot kisi-kisi ini agar lebih jelas dan operasional (kata kerja konkret sesuai cognitive level). Pertahankan KD, materi, dan cognitive level. Pakai update_blueprint_slot.",
  },
  {
    label: "Custom…",
    hint: "Tulis instruksi bebas",
    prompt: "",
    needsInput: true,
    inputHint: "Apa yang ingin dilakukan dengan slot ini?",
  },
];

const DRAFT_COMMANDS: Command[] = [
  {
    label: "Generate soal dari topik",
    hint: "Tulis topik, AI buat soal lengkap",
    prompt: "",
    needsInput: true,
    inputHint: "Topik soal? (mis: konversi suhu Celcius ke Fahrenheit)",
  },
  {
    label: "Soal acak sesuai exam",
    hint: "AI pilih topik berdasarkan konteks exam",
    prompt: `${QUESTION_QUALITY_CONTRACT}\n\nBuat 1 soal baru yang relevan dengan exam ini. Pilih topik yang BELUM pernah ditanyakan di soal existing (cek dulu pakai list_questions atau lihat konteks). Tipe + level kognitif menyesuaikan tema exam. Pakai create_question. Jangan tampilkan kisi-kisi internal kecuali diminta.`,
  },
  {
    label: "Soal dengan stimulus",
    hint: "Bacaan/teks/kasus + N soal yang merujuk",
    prompt: "",
    needsInput: true,
    inputHint: "Topik stimulus + berapa soal? (mis: ekosistem hutan, 3 soal)",
  },
  {
    label: "Buat dari kisi-kisi",
    hint: "Pilih slot blueprint kosong + isi otomatis",
    prompt: `${QUESTION_QUALITY_CONTRACT}\n\nLihat slot blueprint yang masih kosong di exam ini. Pilih satu, lalu generate 1 soal sesuai kisi-kisi slot tersebut (KD/materi/indikator/cognitive level/difficulty). Pakai create_question dengan blueprintSlotId.`,
  },
  {
    label: "Custom…",
    hint: "Tulis instruksi bebas",
    prompt: "",
    needsInput: true,
    inputHint: "Mau buat soal seperti apa?",
  },
];

const EXAM_COMMANDS: Command[] = [
  {
    label: "Generate kisi-kisi dari semua soal",
    hint: "Extract KD/Materi/Indikator untuk seluruh soal",
    prompt: "Untuk setiap soal di exam ini, rumuskan kisi-kisi lengkap. Indikator harus konseptual: lebih umum dari redaksi soal, tidak menyalin frasa soal, tidak menyebut opsi/jawaban benar, dan tidak menjadi bocoran; tulis kompetensi yang diuji guru, bukan petunjuk untuk siswa. PAKAI bulk_apply_question_kisi_kisi SEKALI dengan items untuk semua questionId, replace=false. JANGAN pakai apply_blueprint_analysis / convert_questions_to_kisi_kisi.",
  },
  {
    label: "Tambah N soal random",
    hint: "Generate beberapa soal sesuai tema exam",
    prompt: "",
    needsInput: true,
    inputHint: "Berapa soal dan tipe? (mis: 5 soal multiple_choice level HOTS)",
  },
  {
    label: "Buat section baru",
    hint: "Tambah section dengan judul + deskripsi",
    prompt: "",
    needsInput: true,
    inputHint: "Judul section? (mis: Bagian B - Aljabar)",
  },
  {
    label: "Audit duplikat",
    hint: "Scan soal yang mirip + saran rephrase",
    prompt: "Audit semua soal di exam ini untuk cari paraphrase duplikat. Untuk tiap pair yang similarity >= 0.7 (pakai find_similar_questions per soal jika perlu), sebutkan ID dua soal + similarity score + saran apakah merge / rephrase salah satu / keep both. JANGAN langsung mutate — ini read-only audit.",
  },
  {
    label: "Custom…",
    hint: "Tulis instruksi bebas scope exam",
    prompt: "",
    needsInput: true,
    inputHint: "Apa yang ingin dilakukan terhadap exam ini?",
  },
];

const COMMANDS_BY_KIND: Record<EntityKind, Command[]> = {
  question: QUESTION_COMMANDS,
  group: GROUP_COMMANDS,
  slot: SLOT_COMMANDS,
  draft: DRAFT_COMMANDS,
  exam: EXAM_COMMANDS,
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
  const buttonRef = useRef<HTMLButtonElement | null>(null);
  const [anchor, setAnchor] = useState<{ top: number; left: number } | null>(null);
  const [mounted, setMounted] = useState(false);

  // Portal target only available after mount (SSR guard).
  useEffect(() => { setMounted(true); }, []);

  // Anchor the popover to the button's bounding rect each time we
  // open. Recomputed on resize + scroll so it stays attached even if
  // the parent card scrolls under it.
  useEffect(() => {
    if (!open) { setAnchor(null); return; }
    function compute() {
      const btn = buttonRef.current;
      if (!btn) return;
      const r = btn.getBoundingClientRect();
      // Right-align the 18rem (288px) popover to the button's right edge
      const POPOVER_WIDTH = 288;
      const left = Math.max(8, r.right - POPOVER_WIDTH);
      const top = r.bottom + 6;
      setAnchor({ top, left });
    }
    compute();
    window.addEventListener("resize", compute);
    window.addEventListener("scroll", compute, true);
    return () => {
      window.removeEventListener("resize", compute);
      window.removeEventListener("scroll", compute, true);
    };
  }, [open]);

  // Close on outside click. Uses pointerdown so it fires before the
  // click handler on the trigger button (otherwise re-opening immediately).
  useEffect(() => {
    if (!open) return;
    function onClick(e: MouseEvent) {
      const target = e.target as Node;
      if (popoverRef.current && popoverRef.current.contains(target)) return;
      if (buttonRef.current && buttonRef.current.contains(target)) return;
      closeAll();
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

      // Tell the panel a turn just started so it can render the user
      // bubble + thinking spinner immediately, instead of looking
      // frozen for ~30s while the LLM round-trips. Display label
      // prefers the command name over the verbose system prompt.
      const displayMsg = selectedCmd?.label && !selectedCmd.needsInput
        ? `✨ ${selectedCmd.label}`
        : selectedCmd?.label
        ? `✨ ${selectedCmd.label}: ${inputValue.trim()}`
        : prompt.slice(0, 120);
      window.dispatchEvent(new CustomEvent("morfoschools:ai-turn-started", {
        detail: { displayMessage: displayMsg },
      }));

      const activeEntities: Record<string, string> = {};
      if (examId) activeEntities.examId = examId;
      if (templateId) activeEntities.templateId = templateId;
      if (entityKind === "question") activeEntities.questionId = entityId;
      else if (entityKind === "group") activeEntities.groupId = entityId;
      else if (entityKind === "slot") activeEntities.slotId = entityId;
      else if (entityKind === "exam") {
        // exam-level: examId already set above; nothing entity-specific to add
      }
      else if (entityKind === "draft") {
        // Draft cards have no entity yet — entityId carries the
        // sectionId or groupId scope hint instead. Format:
        // "section:<uuid>" or "group:<uuid>".
        if (entityId.startsWith("section:")) {
          activeEntities.sectionId = entityId.slice("section:".length);
        } else if (entityId.startsWith("group:")) {
          activeEntities.groupId = entityId.slice("group:".length);
        }
      }

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
      const msg = (e as Error).message;
      setError(msg);
      window.dispatchEvent(new CustomEvent("morfoschools:ai-turn-error", {
        detail: { message: msg },
      }));
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
    let finalPrompt: string;
    // Special-case stimulus block command: route to compound atomic
    // tool so the model creates stimulus + group + N questions in
    // ONE transaction (avoids reasoning-model multi-step plan failure).
    if (selectedCmd.label === "Soal dengan stimulus") {
      finalPrompt = `${QUESTION_QUALITY_CONTRACT}\n\nPakai create_stimulus_block (compound atomic) untuk topik berikut: ${trimmed}.

Format output:
- 1 stimulus (passage/teks/kasus) yang relevan dengan topik
- 1 group yang mengikat stimulus
- N soal multiple_choice yang merujuk ke stimulus tersebut (kalau user tidak menyebut N, default 3 soal)
- Setiap soal harus benar-benar membutuhkan pemahaman stimulus, bukan bisa dijawab tanpa membaca stimulus
- Lifecycle stimulus: exam_scoped (default Opsi B)
- JANGAN chain create_stimulus + create_question_group + create_question terpisah — itu akan gagal karena step ke-2 butuh ID dari step ke-1.`;
    } else if (selectedCmd.label === "Buat / ganti stimulus") {
      // Group-scoped: write stimulus snapshot to the EXISTING group
      // (don't create a new group). Pakai update_question_group
      // dengan titleSnapshot + bodySnapshot baru.
      finalPrompt = `Generate teks stimulus untuk group yang sedang difokus, dengan topik: ${trimmed}.

Format output:
- Stimulus title yang descriptive
- Stimulus body: passage/teks/kasus yang factually correct dan relevan untuk dijadikan dasar soal
- Pakai update_question_group dengan titleSnapshot + bodySnapshot baru
- JANGAN buat group baru, JANGAN buat soal di turn ini—cuma stimulus dulu. Soal akan ditambahkan iteratif setelahnya.`;
    } else if (selectedCmd.label === "Generate soal dari topik") {
      finalPrompt = `${QUESTION_QUALITY_CONTRACT}\n\nBuat 1 soal berkualitas berdasarkan topik/permintaan user berikut: ${trimmed}. Gunakan create_question. Jika user tidak menyebut tipe, gunakan multiple_choice. Jangan tampilkan kisi-kisi internal kecuali diminta.`;
    } else if (selectedCmd.label === "Tambah N soal random") {
      finalPrompt = `${QUESTION_QUALITY_CONTRACT}\n\nTambahkan beberapa soal berkualitas ke exam ini berdasarkan permintaan: ${trimmed}. Pilih topik yang relevan dengan exam dan belum duplikatif dengan soal existing. Gunakan batch_create_questions. Variasikan level kognitif/difficulty sesuai permintaan. Jangan tampilkan kisi-kisi internal kecuali diminta.`;
    } else if (selectedCmd.label === "Tambah 1 soal sesuai stimulus") {
      finalPrompt = `${QUESTION_QUALITY_CONTRACT}\n\nTambahkan 1 soal baru ke group yang sedang difokus, yang merujuk ke stimulus group tersebut.

Detail dari user: ${trimmed}

Konstrain:
- Pakai create_question dengan groupId pointing ke group ini (groupId akan ada di FOKUS GROUP block)
- Soal harus genuinely require pembaca untuk paham stimulus untuk menjawab (jangan soal yang bisa dijawab tanpa baca stimulus)
- Cek soal existing di group dulu via FOKUS GROUP — jangan duplikasi sudut pandang/fokus yang sama`;
    } else if (selectedCmd.label === "Tambah N soal ke group") {
      finalPrompt = `${QUESTION_QUALITY_CONTRACT}\n\nTambahkan beberapa soal baru ke group yang sedang difokus, yang merujuk ke stimulus group tersebut.

Detail dari user: ${trimmed}

Konstrain:
- Pakai batch_create_questions dengan groupId pointing ke group ini (groupId ada di FOKUS GROUP block)
- Setiap soal harus genuinely require pembaca untuk paham stimulus
- Variasikan sudut pandang antar soal (jangan semua tanya hal yang sama dari sudut yang sama)
- Cek soal existing di group dulu — jangan duplikasi`;
    } else if (selectedCmd.prompt) {
      finalPrompt = `${selectedCmd.prompt}\n\nDetail dari user: ${trimmed}`;
    } else {
      finalPrompt = trimmed;
    }
    void dispatch(finalPrompt);
  }

  const commands = COMMANDS_BY_KIND[entityKind];

  return (
    <div className={cn("relative inline-block", className)}>
      <button
        ref={buttonRef}
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

      {mounted && open && anchor && createPortal(
        <div
          ref={popoverRef}
          style={{ position: "fixed", top: anchor.top, left: anchor.left, width: 288 }}
          className="z-[100] overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)] shadow-lg"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-center gap-2 border-b border-[var(--border)] bg-[var(--accent)]/30 px-3 py-2">
            <Wand2 className="h-3.5 w-3.5 text-[var(--brand)]" />
            <h4 className="flex-1 text-[11.5px] font-semibold text-[var(--foreground)]">
              AI assist {entityKind === "question" ? "soal" : entityKind === "group" ? "group" : entityKind === "slot" ? "kisi-kisi" : entityKind === "exam" ? "exam" : "soal baru"}
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
        </div>,
        document.body,
      )}
    </div>
  );
}
