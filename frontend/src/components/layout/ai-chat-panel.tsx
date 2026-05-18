"use client";

import * as React from "react";
import { Bot, Loader2, SendHorizontal, Sparkles, X, GraduationCap } from "lucide-react";
import { cn } from "@/lib/cn";

type ChatMessage = {
  role: "user" | "assistant";
  content: string;
};

const MAX_CONTEXT_MESSAGES = 8;
const MAX_CONTEXT_CHARS = 6_000;

function compactMessages(messages: ChatMessage[]) {
  const compacted: ChatMessage[] = [];
  let usedChars = 0;
  for (const msg of [...messages].reverse()) {
    if (compacted.length >= MAX_CONTEXT_MESSAGES || usedChars >= MAX_CONTEXT_CHARS) break;
    const content = msg.content.trim();
    if (!content) continue;
    const remaining = MAX_CONTEXT_CHARS - usedChars;
    compacted.unshift({ ...msg, content: content.slice(0, remaining) });
    usedChars += Math.min(content.length, remaining);
  }
  return compacted;
}

const suggestions = [
  { title: "Cek jadwal ujian", prompt: "Tampilkan jadwal ujian terbaru." },
  { title: "Tambah kelas baru", prompt: "Bantu aku tambah kelas baru." },
  { title: "Buat exam", prompt: "Bantu aku buat exam baru." },
  { title: "Tambah soal", prompt: "Bantu aku tambah soal ke exam." },
];

const initialMessages: ChatMessage[] = [
  { role: "assistant", content: "Halo, saya MORFOSCHOOLS AI Agent. Saya siap membantu operasional sekolah: analisis kelas, persiapan exam, grading, dan drafting komunikasi." },
];

interface AiChatPanelProps {
  open: boolean;
  onClose: () => void;
}

export function AiChatPanel({ open, onClose }: AiChatPanelProps) {
  const [messages, setMessages] = React.useState<ChatMessage[]>(initialMessages);
  const [input, setInput] = React.useState("");
  const [isSending, setIsSending] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const scrollRef = React.useRef<HTMLDivElement | null>(null);

  React.useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" });
  }, [messages, isSending]);

  async function sendMessage(content: string) {
    const trimmed = content.trim();
    if (!trimmed || isSending) return;

    const nextMessages: ChatMessage[] = [...messages, { role: "user", content: trimmed }];
    setMessages(nextMessages);
    setInput("");
    setError(null);
    setIsSending(true);

    try {
      const response = await fetch("/api/ai-chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ messages: compactMessages(nextMessages) }),
      });

      const data = await response.json().catch(() => null);
      if (!response.ok || !data?.message?.content) {
        throw new Error(data?.error ?? "AI agent belum bisa merespons.");
      }

      setMessages((curr) => [...curr, { role: "assistant", content: data.message.content }]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menghubungi AI.");
      setMessages((curr) => curr.slice(0, -1));
      setInput(trimmed);
    } finally {
      setIsSending(false);
    }
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    void sendMessage(input);
  }

  if (!open) return null;

  return (
    <aside className="fixed inset-y-0 right-0 z-50 flex w-full max-w-sm flex-col bg-[var(--shell)] text-white shadow-2xl md:rounded-l-2xl">
      {/* Header */}
      <div className="shrink-0 px-4 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-white/10">
              <Bot className="h-4 w-4" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <p className="text-[13px] font-bold text-[var(--shell-foreground)]">MORFOSCHOOLS AI</p>
                <span className="inline-flex items-center gap-1 rounded-full bg-emerald-400/10 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wider text-emerald-300">
                  <Sparkles className="h-2.5 w-2.5" /> Live
                </span>
              </div>
              <p className="text-[11px] text-[var(--shell-muted)]">School operations assistant</p>
            </div>
          </div>
          <button onClick={onClose} className="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--shell-muted)] hover:text-[var(--shell-foreground)] hover:bg-white/10 transition-colors">
            <X size={16} />
          </button>
        </div>
      </div>

      {/* Messages */}
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-3 space-y-3">
        {/* Suggestions (show when only initial message) */}
        {messages.length <= 1 && (
          <div className="rounded-xl bg-white/[0.04] p-3 space-y-1.5">
            <p className="text-[10px] font-bold uppercase tracking-wider text-[var(--shell-muted)] mb-2">
              <GraduationCap className="inline h-3 w-3 mr-1" /> Suggested
            </p>
            {suggestions.map((s) => (
              <button
                key={s.title}
                type="button"
                disabled={isSending}
                onClick={() => void sendMessage(s.prompt)}
                className="w-full rounded-lg bg-white/[0.04] px-3 py-2 text-left text-[12px] font-medium text-[var(--shell-foreground)] hover:bg-white/[0.08] transition-colors disabled:opacity-50"
              >
                {s.title}
              </button>
            ))}
          </div>
        )}

        {/* Chat messages */}
        {messages.map((msg, i) => (
          <div key={i} className={cn("flex", msg.role === "user" ? "justify-end" : "justify-start")}>
            <div className={cn(
              "max-w-[85%] rounded-2xl px-3.5 py-2.5 text-[12px] leading-relaxed",
              msg.role === "user"
                ? "rounded-tr-md bg-[#486b9c] text-white"
                : "rounded-tl-md bg-white/[0.06] text-[var(--shell-foreground)]"
            )}>
              <p className="whitespace-pre-wrap">{msg.content}</p>
            </div>
          </div>
        ))}

        {isSending && (
          <div className="flex justify-start">
            <div className="inline-flex items-center gap-2 rounded-2xl rounded-tl-md bg-white/[0.06] px-3.5 py-2.5 text-[12px] text-[var(--shell-muted)]">
              <Loader2 className="h-3.5 w-3.5 animate-spin" /> Berpikir...
            </div>
          </div>
        )}
      </div>

      {/* Error */}
      {error && (
        <div className="mx-4 mb-2 rounded-lg bg-red-400/10 border border-red-400/20 px-3 py-2 text-[11px] text-red-200">
          {error}
        </div>
      )}

      {/* Input */}
      <form onSubmit={handleSubmit} className="shrink-0 p-4 pt-2">
        <div className="flex items-end gap-2 rounded-xl bg-white/[0.06] p-2">
          <textarea
            rows={2}
            value={input}
            disabled={isSending}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                void sendMessage(input);
              }
            }}
            placeholder="Tanya tentang ujian, kelas, siswa..."
            className="min-h-[48px] flex-1 resize-none bg-transparent px-2 py-1.5 text-[12px] leading-relaxed text-[var(--shell-foreground)] outline-none placeholder:text-[var(--shell-muted)] disabled:opacity-50"
          />
          <button
            type="submit"
            disabled={isSending || !input.trim()}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-white text-[var(--shell)] hover:bg-white/90 disabled:opacity-40 transition-colors"
          >
            {isSending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <SendHorizontal className="h-3.5 w-3.5" />}
          </button>
        </div>
      </form>
    </aside>
  );
}
