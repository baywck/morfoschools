"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Check, Loader2, Sparkles, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

type MagicProposal = {
  proposalId: string;
  toolName: string;
  confirmationText: string;
  expiresAt?: string;
};

type QuestionOptionPreview = { content: string; isCorrect?: boolean };
type QuestionPatchPreview = {
  content?: string;
  explanation?: string;
  options?: QuestionOptionPreview[];
  competencyCode?: string;
  materi?: string;
  indikator?: string;
  cognitiveLevel?: string;
  difficulty?: string;
};
type GroupPatchPreview = { titleSnapshot?: string; bodySnapshot?: string };
type StructuredPreview = {
  groupPatch?: GroupPatchPreview | null;
  questionPatch?: QuestionPatchPreview;
  kisiKisiPatch?: Record<string, unknown>;
};

type MagicModalDetail = {
  entityKind?: "question" | "group";
  mode?: "edit" | "create";
  questionId?: string;
  groupId?: string;
  action: string;
  instruction: string;
  title?: string;
};

function csrfToken() {
  const match = document.cookie.match(/csrf_token=([^;]+)/);
  return match ? match[1] : "";
}

function splitSections(markdown: string) {
  const content = markdown || "";
  const target = content.match(/Target:\s*`([^`]+)`/)?.[1] ?? "";
  const stimulus = content.match(/\*\*Stimulus\/group baru:\*\*\n([\s\S]*?)(?:\n\n\*\*|$)/)?.[1]?.replace(/\n> /g, "\n").replace(/^> /, "").trim() ?? "";
  const newContent = content.match(/\*\*Konten baru:\*\*\n> ([\s\S]*?)(?:\n\n\*\*|$)/)?.[1]?.replace(/\n> /g, "\n").trim() ?? "";
  const explanation = content.match(/\*\*Pembahasan baru:\*\*\s*([\s\S]*?)(?:\n\n\*\*|$)/)?.[1]?.trim() ?? "";
  const kisiBlock = content.match(/\*\*Update kisi-kisi:\*\*\n([\s\S]*?)(?:\n\n\*\*|$)/)?.[1] ?? "";
  const kisi = kisiBlock.split("\n").map((line) => line.trim()).filter(Boolean);
  const optionsBlock = content.match(/\*\*Opsi baru \((\d+)\):\*\*\n([\s\S]*)/)?.[2] ?? "";
  const options = optionsBlock.split("\n").map((line) => line.trim()).filter(Boolean);
  return { target, stimulus, newContent, explanation, kisi, options };
}

export function QuestionMagicModalHost() {
  const [mounted, setMounted] = useState(false);
  const [detail, setDetail] = useState<MagicModalDetail | null>(null);
  const [proposal, setProposal] = useState<MagicProposal | null>(null);
  const [current, setCurrent] = useState<Record<string, unknown> | null>(null);
  const [suggested, setSuggested] = useState<StructuredPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [approving, setApproving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => setMounted(true), []);

  useEffect(() => {
    function onOpen(e: Event) {
      const next = (e as CustomEvent).detail as MagicModalDetail;
      setDetail(next);
      setProposal(null);
      setError(null);
      void generate(next);
    }
    window.addEventListener("morfoschools:question-magic-open", onOpen);
    return () => window.removeEventListener("morfoschools:question-magic-open", onOpen);
  }, []);

  async function generate(next: MagicModalDetail) {
    setLoading(true);
    setError(null);
    try {
      const endpoint = next.entityKind === "group" ? "/api/v1/ai/group-actions" : "/api/v1/ai/question-actions";
      const res = await fetch(`${API_BASE}${endpoint}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() },
        credentials: "include",
        body: JSON.stringify({
          requestId: crypto.randomUUID?.() ?? `${Date.now()}`,
          ...(next.entityKind === "group" ? { groupId: next.groupId } : { questionId: next.questionId }),
          action: next.action,
          instruction: next.instruction,
        }),
      });
      const data = await res.json().catch(() => null);
      if (!res.ok) throw new Error(data?.error?.message || "Gagal membuat saran AI.");
      setProposal(data?.proposal ?? null);
      setCurrent(data?.current ?? null);
      setSuggested(data?.suggested ?? null);
    } catch (e) {
      setError((e as Error).message || "Gagal membuat saran AI.");
    } finally {
      setLoading(false);
    }
  }

  async function approve() {
    if (!proposal) return;
    setApproving(true);
    setError(null);
    try {
      const res = await fetch(`${API_BASE}/api/v1/ai/confirm`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken() },
        credentials: "include",
        body: JSON.stringify({ proposalId: proposal.proposalId }),
      });
      const data = await res.json().catch(() => null);
      if (!res.ok) throw new Error(data?.error?.message || "Gagal menerapkan perubahan.");
      window.dispatchEvent(new Event("morfoschools:data-changed"));
      setDetail(null);
      setProposal(null);
    } catch (e) {
      setError((e as Error).message || "Gagal menerapkan perubahan.");
    } finally {
      setApproving(false);
    }
  }

  if (!mounted || !detail) return null;
  const sections = proposal ? splitSections(proposal.confirmationText) : null;
  const questionPatch = suggested?.questionPatch;
  const groupPatch = suggested?.groupPatch ?? (suggested as { groupPatch?: GroupPatchPreview } | null)?.groupPatch;
  const kisiPatch = suggested?.kisiKisiPatch;

  return createPortal(
    <div className="fixed inset-0 z-[90] flex items-center justify-center bg-black/35 p-4 backdrop-blur-[2px]">
      <div className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--border)] bg-[var(--card)] shadow-[0_30px_120px_rgba(0,0,0,0.32)]">
        <div className="flex items-start justify-between gap-4 border-b border-[var(--border)] px-5 py-4">
          <div className="flex items-start gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-[var(--border)] bg-[var(--muted)] text-[var(--foreground)]">
              <Sparkles className="h-4 w-4" />
            </div>
            <div>
              <div className="text-sm font-semibold text-[var(--foreground)]">AI inline edit soal</div>
              <div className="mt-0.5 text-xs text-[var(--muted-foreground)]">{detail.title ?? "Preview perubahan sebelum diterapkan"}</div>
            </div>
          </div>
          <button className="rounded-lg p-2 text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]" onClick={() => setDetail(null)} disabled={approving}>
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {loading && (
            <div className="flex min-h-64 flex-col items-center justify-center gap-3 text-center">
              <Loader2 className="h-6 w-6 animate-spin text-[var(--muted-foreground)]" />
              <div className="text-sm font-medium">Membuat saran perubahan…</div>
              <div className="max-w-sm text-xs text-[var(--muted-foreground)]">AI membaca soal fokus dan menyiapkan patch terarah. Ini tidak masuk ke chatbot exam.</div>
            </div>
          )}

          {error && !loading && (
            <div className="rounded-xl border border-[var(--danger)] bg-[var(--danger-soft)] p-4 text-sm text-[var(--foreground)]">
              <div className="font-semibold">Gagal membuat/menjalankan saran</div>
              <div className="mt-1 text-xs text-[var(--muted-foreground)]">{error}</div>
            </div>
          )}

          {proposal && sections && !loading && (
            <div className="space-y-4">
              <div className="rounded-2xl border border-[var(--border)] bg-[var(--accent)] p-3 text-xs text-[var(--muted-foreground)]">
                Target {detail.entityKind === "group" ? "group" : "soal"}: <span className="font-mono text-[var(--foreground)]">{sections.target || detail.questionId || detail.groupId}</span>
              </div>

              {(groupPatch?.bodySnapshot || groupPatch?.titleSnapshot) && (
                <section className="grid gap-3 md:grid-cols-2">
                  <div className="rounded-xl border border-[var(--border)] bg-[var(--background)] p-4">
                    <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Stimulus saat ini</div>
                    <div className="max-h-[46vh] overflow-y-auto whitespace-pre-line break-words rounded-xl bg-[var(--muted)]/35 p-3 text-sm leading-7 text-[var(--muted-foreground)]">{String(current?.groupBody ?? current?.bodySnapshot ?? "Belum ada stimulus group.")}</div>
                  </div>
                  <div className="rounded-xl border border-[var(--success)] bg-[var(--success-soft)] p-4">
                    <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Stimulus saran</div>
                    {groupPatch.titleSnapshot && <div className="mb-2 text-sm font-semibold text-[var(--foreground)]">{groupPatch.titleSnapshot}</div>}
                    <div className="max-h-[46vh] overflow-y-auto whitespace-pre-line break-words rounded-xl bg-[var(--background)]/55 p-3 text-sm leading-7 text-[var(--foreground)]">{groupPatch.bodySnapshot}</div>
                  </div>
                </section>
              )}

              {questionPatch?.content && (
                detail.mode === "create" || !current?.content ? (
                  <section className="rounded-2xl border border-[var(--success)] bg-[var(--success-soft)] p-4">
                    <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Soal baru</div>
                    <div className="max-h-[42vh] overflow-y-auto whitespace-pre-line break-words rounded-xl bg-[var(--background)]/55 p-4 text-sm leading-7 text-[var(--foreground)]">{questionPatch.content}</div>
                  </section>
                ) : (
                  <section className="grid gap-3 md:grid-cols-2">
                    <div className="rounded-xl border border-[var(--border)] bg-[var(--background)] p-4">
                      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Soal saat ini</div>
                      <div className="max-h-[34vh] overflow-y-auto whitespace-pre-line break-words rounded-xl bg-[var(--muted)]/35 p-3 text-sm leading-7 text-[var(--muted-foreground)]">{String(current?.content ?? "")}</div>
                    </div>
                    <div className="rounded-xl border border-[var(--success)] bg-[var(--success-soft)] p-4">
                      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Soal saran</div>
                      <div className="max-h-[34vh] overflow-y-auto whitespace-pre-line break-words rounded-xl bg-[var(--background)]/55 p-3 text-sm leading-7 text-[var(--foreground)]">{questionPatch.content}</div>
                    </div>
                  </section>
                )
              )}

              {sections.stimulus && !groupPatch?.bodySnapshot && (
                <section className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
                  <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Stimulus / group baru</div>
                  <div className="whitespace-pre-wrap text-sm leading-6 text-[var(--foreground)]">{sections.stimulus}</div>
                </section>
              )}

              {sections.newContent && !questionPatch?.content && (
                <section className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
                  <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Konten baru</div>
                  <div className="whitespace-pre-wrap text-sm leading-6 text-[var(--foreground)]">{sections.newContent}</div>
                </section>
              )}

              {(kisiPatch || sections.kisi.length > 0) && (
                <section className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
                  <div className="mb-3 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Kisi-kisi / metadata</div>
                  {kisiPatch ? (
                    <div className="grid gap-2 sm:grid-cols-2">
                      {Object.entries(kisiPatch).filter(([, value]) => value !== null && value !== undefined && value !== "").map(([key, value]) => (
                        <div key={key} className="rounded-lg border border-[var(--border)] bg-[var(--background)] p-3">
                          <div className="text-[10px] font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">{key}</div>
                          <div className="mt-1 text-sm text-[var(--foreground)]">{String(value)}</div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="grid gap-2 sm:grid-cols-2">
                      {sections.kisi.map((row, idx) => {
                        const [label, ...rest] = row.replace(/^[-•]\s*/, "").split(":");
                        return (
                          <div key={idx} className="rounded-lg border border-[var(--border)] bg-[var(--background)] p-3">
                            <div className="text-[10px] font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">{label.trim()}</div>
                            <div className="mt-1 text-sm text-[var(--foreground)]">{rest.join(":").trim()}</div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </section>
              )}


              {(questionPatch?.options?.length || (sections?.options.length ?? 0) > 0) && (
                <section className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
                  <div className="mb-3 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Opsi jawaban</div>
                  <div className="space-y-2">
                    {(questionPatch?.options ?? sections.options.map((opt) => ({ content: opt.replace(/^[A-E]\)\s*/, "").replace("✅", "").trim(), isCorrect: opt.includes("✅") }))).map((opt, idx) => {
                      const correct = Boolean(opt.isCorrect);
                      return (
                        <div key={idx} className={cn("rounded-xl border p-3 text-sm", correct ? "border-[var(--success)] bg-[var(--success-soft)]" : "border-[var(--border)] bg-[var(--background)]")}>
                          <div className="grid grid-cols-[1.75rem_1fr] items-start gap-3">
                            <span className={cn("mt-0.5 flex h-5 w-5 items-center justify-center rounded-md border", correct ? "border-[var(--success)] bg-[var(--success)] text-white" : "border-[var(--border)] bg-[var(--card)] text-transparent")}>
                              <Check className="h-3.5 w-3.5" />
                            </span>
                            <span className="min-w-0 whitespace-pre-line break-words leading-6 text-left text-[var(--foreground)]">{opt.content}</span>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </section>
              )}

              {(questionPatch?.explanation || sections.explanation) && (
                <section className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
                  <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">Pembahasan</div>
                  <div className="whitespace-pre-wrap text-sm leading-6 text-[var(--foreground)]">{questionPatch?.explanation ?? sections.explanation}</div>
                </section>
              )}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between gap-3 border-t border-[var(--border)] px-5 py-4">
          <div className="text-xs text-[var(--muted-foreground)]">Tidak dikirim ke chatbot. Approve akan mengganti {detail.entityKind === "group" ? "stimulus group" : "soal terkait"} via proposal aman.</div>
          <div className="flex gap-2">
            <Button type="button" variant="secondary" onClick={() => setDetail(null)} disabled={approving}>Batal</Button>
            {proposal && <Button type="button" onClick={approve} loading={approving}>Terapkan</Button>}
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
