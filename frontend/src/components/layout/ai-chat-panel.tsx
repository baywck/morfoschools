"use client";

import * as React from "react";
import { useState, useRef, useEffect } from "react";
import { Bot, Loader2, SendHorizontal, Sparkles, X, GraduationCap, Plus, Paperclip, Image, FileCode, ChevronDown, Check, Zap, Brain } from "lucide-react";
import { cn } from "@/lib/cn";

type ChatMessage = {
  role: "user" | "assistant";
  content: string;
};

// --- Model Selector ---

interface Model {
  id: string;
  name: string;
  description: string;
  icon: React.ReactNode;
  badge?: string;
}

const models: Model[] = [
  { id: "morfoschools", name: "Morfoschools", description: "Default school AI", icon: <Zap className="h-3.5 w-3.5 text-[var(--brand)]" />, badge: "Default" },
  { id: "gpt-4o", name: "GPT-4o", description: "OpenAI flagship", icon: <Sparkles className="h-3.5 w-3.5 text-emerald-400" /> },
  { id: "claude", name: "Claude", description: "Anthropic", icon: <Brain className="h-3.5 w-3.5 text-purple-400" /> },
];

function ModelSelector() {
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState(models[0]);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-1.5 px-2 py-1 rounded-lg text-[11px] font-medium text-[var(--shell-muted)] hover:text-[var(--shell-foreground)] hover:bg-white/[0.06] transition-colors"
      >
        {selected.icon}
        <span>{selected.name}</span>
        <ChevronDown className={cn("h-3 w-3 transition-transform", open && "rotate-180")} />
      </button>

      {open && (
        <div className="absolute bottom-full left-0 mb-1 w-48 rounded-xl border border-white/10 bg-[#1a1a1e]/95 backdrop-blur-xl p-1 shadow-lg z-50">
          <p className="px-2.5 py-1 text-[9px] font-semibold uppercase tracking-wider text-[var(--shell-muted)]">Select Model</p>
          {models.map((model) => (
            <button
              key={model.id}
              onClick={() => { setSelected(model); setOpen(false); }}
              className={cn(
                "w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-left transition-colors",
                selected.id === model.id ? "bg-white/10 text-white" : "text-[var(--shell-muted)] hover:bg-white/[0.06] hover:text-white"
              )}
            >
              {model.icon}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="text-[11px] font-medium">{model.name}</span>
                  {model.badge && <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-[var(--brand)]/20 text-[var(--brand)]">{model.badge}</span>}
                </div>
                <span className="text-[10px] text-[var(--shell-muted)]">{model.description}</span>
              </div>
              {selected.id === model.id && <Check className="h-3.5 w-3.5 text-[var(--brand)]" />}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// --- Attach Menu ---

function AttachMenu() {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex h-7 w-7 items-center justify-center rounded-lg bg-white/[0.08] text-[var(--shell-muted)] hover:bg-white/[0.12] hover:text-[var(--shell-foreground)] transition-colors"
      >
        <Plus className={cn("h-3.5 w-3.5 transition-transform", open && "rotate-45")} />
      </button>

      {open && (
        <div className="absolute bottom-full left-0 mb-1 w-40 rounded-xl border border-white/10 bg-[#1a1a1e]/95 backdrop-blur-xl p-1 shadow-lg z-50">
          {[
            { icon: <Paperclip className="h-3.5 w-3.5" />, label: "Upload file" },
            { icon: <Image className="h-3.5 w-3.5" />, label: "Add image" },
            { icon: <FileCode className="h-3.5 w-3.5" />, label: "Import code" },
          ].map((item, i) => (
            <button key={i} onClick={() => setOpen(false)} className="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-[11px] text-[var(--shell-muted)] hover:bg-white/[0.06] hover:text-white transition-colors">
              {item.icon}
              {item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// --- Main Panel ---

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
  const [messages, setMessages] = useState<ChatMessage[]>(initialMessages);
  const [input, setInput] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" });
  }, [messages, isSending]);

  // Auto-resize textarea
  useEffect(() => {
    const ta = textareaRef.current;
    if (ta) {
      ta.style.height = "auto";
      ta.style.height = `${Math.min(ta.scrollHeight, 120)}px`;
    }
  }, [input]);

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
    <aside className="fixed inset-y-0 right-0 z-50 flex w-[360px] flex-col bg-[var(--shell)] text-white">
      {/* Header */}
      <div className="shrink-0 px-4 py-3 border-b border-white/[0.06]">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-white/[0.08]">
              <Bot className="h-4 w-4 text-[var(--shell-foreground)]" />
            </div>
            <div>
              <div className="flex items-center gap-1.5">
                <p className="text-[12px] font-bold text-[var(--shell-foreground)]">MORFOSCHOOLS AI</p>
                <span className="inline-flex items-center gap-0.5 rounded-full bg-emerald-400/10 px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-wider text-emerald-300">
                  <Sparkles className="h-2 w-2" /> Live
                </span>
              </div>
              <p className="text-[10px] text-[var(--shell-muted)]">School operations assistant</p>
            </div>
          </div>
          <button onClick={onClose} className="flex h-7 w-7 items-center justify-center rounded-lg text-[var(--shell-muted)] hover:text-[var(--shell-foreground)] hover:bg-white/[0.06] transition-colors">
            <X size={14} />
          </button>
        </div>
      </div>

      {/* Messages */}
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-3 space-y-3">
        {/* Suggestions */}
        {messages.length <= 1 && (
          <div className="rounded-xl bg-white/[0.03] p-3 space-y-1.5">
            <p className="text-[9px] font-bold uppercase tracking-wider text-[var(--shell-muted)] mb-2">
              <GraduationCap className="inline h-3 w-3 mr-1" /> Suggested
            </p>
            {suggestions.map((s) => (
              <button
                key={s.title}
                type="button"
                disabled={isSending}
                onClick={() => void sendMessage(s.prompt)}
                className="w-full rounded-lg bg-white/[0.04] px-3 py-2 text-left text-[11px] font-medium text-[var(--shell-foreground)] hover:bg-white/[0.08] transition-colors disabled:opacity-50"
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
                ? "rounded-tr-md bg-[var(--brand)] text-white"
                : "rounded-tl-md bg-white/[0.06] text-[var(--shell-foreground)]"
            )}>
              <p className="whitespace-pre-wrap">{msg.content}</p>
            </div>
          </div>
        ))}

        {isSending && (
          <div className="flex justify-start">
            <div className="inline-flex items-center gap-2 rounded-2xl rounded-tl-md bg-white/[0.06] px-3.5 py-2.5 text-[11px] text-[var(--shell-muted)]">
              <Loader2 className="h-3.5 w-3.5 animate-spin" /> Berpikir...
            </div>
          </div>
        )}
      </div>

      {/* Error */}
      {error && (
        <div className="mx-4 mb-2 rounded-lg bg-red-400/10 border border-red-400/20 px-3 py-2 text-[10px] text-red-200">
          {error}
        </div>
      )}

      {/* Input area */}
      <form onSubmit={handleSubmit} className="shrink-0 p-3 pt-0">
        <div className="rounded-xl bg-white/[0.05] ring-1 ring-white/[0.08]">
          <textarea
            ref={textareaRef}
            rows={1}
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
            className="w-full resize-none bg-transparent px-3.5 pt-3 pb-1.5 text-[12px] leading-relaxed text-[var(--shell-foreground)] outline-none placeholder:text-[var(--shell-muted)] disabled:opacity-50 min-h-[36px] max-h-[120px]"
          />
          {/* Toolbar */}
          <div className="flex items-center justify-between px-2.5 pb-2.5 pt-0.5">
            <div className="flex items-center gap-1">
              <AttachMenu />
              <ModelSelector />
            </div>
            <button
              type="submit"
              disabled={isSending || !input.trim()}
              className="flex h-7 items-center gap-1.5 rounded-lg bg-[var(--brand)] px-2.5 text-[11px] font-semibold text-white hover:opacity-90 disabled:opacity-40 transition-all active:scale-[0.97]"
            >
              {isSending ? <Loader2 className="h-3 w-3 animate-spin" /> : <SendHorizontal className="h-3 w-3" />}
              <span className="hidden sm:inline">Send</span>
            </button>
          </div>
        </div>
      </form>
    </aside>
  );
}
