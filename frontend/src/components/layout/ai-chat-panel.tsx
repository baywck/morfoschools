"use client";

import * as React from "react";
import { useState, useRef, useEffect } from "react";
import { Bot, Loader2, SendHorizontal, Sparkles, X, GraduationCap, Plus, Paperclip, Image, FileCode, ChevronDown, Check, Zap, Brain } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { cn } from "@/lib/cn";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

type ChatMessage = {
  role: "user" | "assistant";
  content: string;
  proposal?: {
    proposalId: string;
    toolName: string;
    confirmationText: string;
    expiresAt: string;
  };
  proposalStatus?: "pending" | "confirmed" | "cancelled";
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
  { id: "gpt-4o", name: "GPT-4o", description: "OpenAI flagship", icon: <Sparkles className="h-3.5 w-3.5 text-[var(--success)]" /> },
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
        className="flex items-center gap-1.5 px-2 py-1 rounded-lg text-[11px] font-medium text-[var(--shell-muted)] hover:text-[var(--shell-foreground)] hover:bg-[var(--shell-hover)] transition-colors"
      >
        {selected.icon}
        <span>{selected.name}</span>
        <ChevronDown className={cn("h-3 w-3 transition-transform", open && "rotate-180")} />
      </button>

      {open && (
        <div className="absolute bottom-full left-0 mb-1 w-48 rounded-xl border border-[var(--shell-border,var(--border))] bg-[var(--shell-elevated,var(--card))]/95 backdrop-blur-xl p-1 shadow-lg z-50">
          <p className="px-2.5 py-1 text-[9px] font-semibold uppercase tracking-wider text-[var(--shell-muted)]">Select Model</p>
          {models.map((model) => (
            <button
              key={model.id}
              onClick={() => { setSelected(model); setOpen(false); }}
              className={cn(
                "w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-left transition-colors",
                selected.id === model.id ? "bg-[var(--shell-active)] text-[var(--shell-foreground)]" : "text-[var(--shell-muted)] hover:bg-[var(--shell-hover)] hover:text-[var(--shell-foreground)]"
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
        className="flex h-7 w-7 items-center justify-center rounded-lg bg-[var(--shell-active)] text-[var(--shell-muted)] hover:bg-[var(--shell-hover)] hover:text-[var(--shell-foreground)] transition-colors"
      >
        <Plus className={cn("h-3.5 w-3.5 transition-transform", open && "rotate-45")} />
      </button>

      {open && (
        <div className="absolute bottom-full left-0 mb-1 w-40 rounded-xl border border-[var(--shell-border,var(--border))] bg-[var(--shell-elevated,var(--card))]/95 backdrop-blur-xl p-1 shadow-lg z-50">
          {[
            { icon: <Paperclip className="h-3.5 w-3.5" />, label: "Upload file" },
            { icon: <Image className="h-3.5 w-3.5" />, label: "Add image" },
            { icon: <FileCode className="h-3.5 w-3.5" />, label: "Import code" },
          ].map((item, i) => (
            <button key={i} onClick={() => setOpen(false)} className="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-[11px] text-[var(--shell-muted)] hover:bg-[var(--shell-hover)] hover:text-[var(--shell-foreground)] transition-colors">
              {item.icon}
              {item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// --- Message Renderer ---

function renderMessageContent(content: string) {
  const lines = content.split("\n");
  const elements: React.ReactNode[] = [];
  let listItems: string[] = [];

  function flushList() {
    if (listItems.length > 0) {
      elements.push(
        <ul key={`list-${elements.length}`} className="my-1.5 space-y-1 pl-3">
          {listItems.map((item, i) => (
            <li key={i} className="flex gap-2 items-start">
              <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--brand)] opacity-70" />
              <span>{renderInline(item)}</span>
            </li>
          ))}
        </ul>
      );
      listItems = [];
    }
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const listMatch = line.match(/^\s*(?:[-*•]|\d+[.)]) (.+)/);

    if (listMatch) {
      listItems.push(listMatch[1]);
    } else {
      flushList();
      if (line.trim() === "") {
        elements.push(<div key={`br-${i}`} className="h-2" />);
      } else {
        elements.push(<p key={`p-${i}`}>{renderInline(line)}</p>);
      }
    }
  }
  flushList();

  return <>{elements}</>;
}

function renderInline(text: string): React.ReactNode {
  // Handle **bold** and *italic*
  const parts: React.ReactNode[] = [];
  const regex = /(\*\*(.+?)\*\*|\*(.+?)\*|`(.+?)`)/g;
  let lastIndex = 0;
  let match;

  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      parts.push(text.slice(lastIndex, match.index));
    }
    if (match[2]) {
      parts.push(<strong key={match.index} className="font-semibold">{match[2]}</strong>);
    } else if (match[3]) {
      parts.push(<em key={match.index}>{match[3]}</em>);
    } else if (match[4]) {
      parts.push(<code key={match.index} className="rounded bg-[var(--shell-input-border)] px-1 py-0.5 text-[10px] font-mono">{match[4]}</code>);
    }
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex));
  }

  return parts.length > 0 ? <>{parts}</> : text;
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
  const pathname = usePathname();
  const router = useRouter();
  const [messages, setMessages] = useState<ChatMessage[]>(initialMessages);
  const [sessionId, setSessionId] = useState<string | null>(() => {
    if (typeof window !== "undefined") return localStorage.getItem("morfoschools-ai-session");
    return null;
  });
  const [input, setInput] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const restoredRef = useRef(false);

  // Restore messages from session on mount
  useEffect(() => {
    if (restoredRef.current || !sessionId) return;
    restoredRef.current = true;
    async function restore() {
      try {
        const res = await fetch(`${API_BASE}/api/v1/ai/sessions/${sessionId}/messages`, { credentials: "include" });
        if (!res.ok) { setSessionId(null); localStorage.removeItem("morfoschools-ai-session"); return; }
        const data = await res.json();
        if (data?.data?.length > 0) {
          setMessages(data.data.map((m: any) => {
            let content = m.content;
            // Parse JSON results into readable messages
            if (content.startsWith("{")) {
              try {
                const parsed = JSON.parse(content);
                if (parsed.success && parsed.message) {
                  content = `✅ ${parsed.message}`;
                } else if (parsed.error) {
                  content = `❌ ${parsed.error}`;
                }
              } catch { /* keep original */ }
            }
            return { role: m.role, content };
          }));
        }
      } catch { /* ignore */ }
    }
    restore();
  }, [sessionId]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" });
  }, [messages, isSending, open]);

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
      const csrfMatch = document.cookie.match(/csrf_token=([^;]+)/);
      const csrfToken = csrfMatch ? csrfMatch[1] : "";

      const response = await fetch(`${API_BASE}/api/v1/ai/chat`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrfToken,
        },
        credentials: "include",
        body: JSON.stringify({
          sessionId: sessionId || undefined,
          message: trimmed,
          shadow: {
            route: pathname,
            activeEntities: parseActiveEntities(pathname),
          },
        }),
      });

      const data = await response.json().catch(() => null);
      if (!response.ok || !data?.message?.content) {
        throw new Error(data?.error?.message || data?.error || "AI agent belum bisa merespons.");
      }

      if (data.sessionId) {
        setSessionId(data.sessionId);
        localStorage.setItem("morfoschools-ai-session", data.sessionId);
      }

      // Check if response contains a proposal (confirmation required)
      const content = data.message.content;
      let proposals: any[] = [];
      if (data.proposal) {
        proposals = [data.proposal];
      } else if (data.proposals && data.proposals.length > 0) {
        proposals = data.proposals;
      }

      if (proposals.length === 0) {
        setMessages((curr) => [...curr, { role: "assistant", content }]);
        // If the response indicates a successful mutation (no proposal = direct execution),
        // trigger data refresh so the app shell updates
        if (data.mutated) {
          router.refresh();
          window.dispatchEvent(new Event("morfoschools:data-changed"));
        }
      } else {
        // First proposal attached to main message
        setMessages((curr) => [...curr, { role: "assistant", content, proposal: proposals[0], proposalStatus: "pending" }]);
        // Additional proposals as separate messages
        for (let i = 1; i < proposals.length; i++) {
          setMessages((curr) => [...curr, { role: "assistant", content: proposals[i].confirmationText, proposal: proposals[i], proposalStatus: "pending" }]);
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menghubungi AI.");
      setMessages((curr) => curr.slice(0, -1));
      setInput(trimmed);
    } finally {
      setIsSending(false);
    }
  }

  async function handleConfirm(proposalId: string, msgIndex: number) {
    try {
      const csrfMatch = document.cookie.match(/csrf_token=([^;]+)/);
      const csrfToken = csrfMatch ? csrfMatch[1] : "";

      const response = await fetch(`${API_BASE}/api/v1/ai/confirm`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
        credentials: "include",
        body: JSON.stringify({ proposalId }),
      });

      const data = await response.json().catch(() => null);
      if (!response.ok) {
        setError(data?.error?.message || "Gagal mengeksekusi aksi.");
        return;
      }

      // Update message status
      setMessages((curr) => curr.map((m, i) => i === msgIndex ? { ...m, proposalStatus: "confirmed" as const } : m));

      // Add result message
      const resultMsg = data?.result?.message || "Aksi berhasil dieksekusi.";
      setMessages((curr) => [...curr, { role: "assistant", content: `✅ ${resultMsg}` }]);

      // Refresh current page data
      router.refresh();
      window.dispatchEvent(new Event("morfoschools:data-changed"));
    } catch {
      setError("Gagal menghubungi server.");
    }
  }

  async function handleCancel(proposalId: string, msgIndex: number) {
    try {
      const csrfMatch = document.cookie.match(/csrf_token=([^;]+)/);
      const csrfToken = csrfMatch ? csrfMatch[1] : "";

      await fetch(`${API_BASE}/api/v1/ai/cancel`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
        credentials: "include",
        body: JSON.stringify({ proposalId }),
      });

      setMessages((curr) => curr.map((m, i) => i === msgIndex ? { ...m, proposalStatus: "cancelled" as const } : m));
    } catch {
      setError("Gagal membatalkan aksi.");
    }
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    void sendMessage(input);
  }

  if (!open) return null;

  return (
    <aside className="fixed inset-0 z-50 flex flex-col bg-[var(--shell)] text-[var(--shell-foreground)] md:inset-y-0 md:left-auto md:right-0 md:w-[360px] md:bg-transparent">
      {/* Header */}
      <div className="shrink-0 px-4 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-[var(--shell-active)]">
              <Bot className="h-4 w-4 text-[var(--shell-foreground)]" />
            </div>
            <div>
              <div className="flex items-center gap-1.5">
                <p className="text-[12px] font-bold text-[var(--shell-foreground)]">MORFOSCHOOLS AI</p>
                <span className="inline-flex items-center gap-0.5 rounded-full bg-[var(--success)]/10 px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-wider text-[var(--success)]">
                  <Sparkles className="h-2 w-2" /> Live
                </span>
              </div>
              <p className="text-[10px] text-[var(--shell-muted)]">School operations assistant</p>
            </div>
          </div>
          <button onClick={onClose} className="flex h-7 w-7 items-center justify-center rounded-lg text-[var(--shell-muted)] hover:text-[var(--shell-foreground)] hover:bg-[var(--shell-hover)] transition-colors">
            <X size={14} />
          </button>
        </div>
      </div>

      {/* Messages */}
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-3 space-y-3">
        {/* Suggestions */}
        {messages.length <= 1 && (
          <div className="rounded-xl bg-[var(--shell-input-bg)] border border-[var(--shell-input-border)] p-3 space-y-1.5">
            <p className="text-[9px] font-bold uppercase tracking-wider text-[var(--shell-muted)] mb-2">
              <GraduationCap className="inline h-3 w-3 mr-1" /> Suggested
            </p>
            {suggestions.map((s) => (
              <button
                key={s.title}
                type="button"
                disabled={isSending}
                onClick={() => void sendMessage(s.prompt)}
                className="w-full rounded-lg bg-[var(--shell-input-bg)] border border-[var(--shell-input-border)] px-3 py-2 text-left text-[11px] font-medium text-[var(--shell-foreground)] hover:border-[var(--brand)] transition-colors disabled:opacity-50"
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
                : "rounded-tl-md bg-[var(--shell-input-bg)] border border-[var(--shell-input-border)] text-[var(--shell-foreground)]"
            )}>
              <div className="whitespace-pre-wrap [&_strong]:font-semibold [&_em]:italic">
                {renderMessageContent(msg.content)}
              </div>

              {/* Confirmation card */}
              {msg.proposal && msg.proposalStatus === "pending" && (
                <div className="mt-2.5 pt-2.5 border-t border-[var(--shell-input-border)]">
                  <p className="text-[11px] font-medium text-[var(--shell-muted)] mb-2">{msg.proposal.confirmationText}</p>
                  <div className="flex gap-2">
                    <button
                      onClick={() => handleConfirm(msg.proposal!.proposalId, i)}
                      className="flex-1 h-7 rounded-lg bg-[var(--brand)] text-[11px] font-semibold text-white hover:opacity-90 active:scale-[0.97] transition-all"
                    >
                      Konfirmasi
                    </button>
                    <button
                      onClick={() => handleCancel(msg.proposal!.proposalId, i)}
                      className="flex-1 h-7 rounded-lg bg-[var(--shell-input-bg)] border border-[var(--shell-input-border)] text-[11px] font-medium text-[var(--shell-muted)] hover:text-[var(--shell-foreground)] transition-colors"
                    >
                      Batal
                    </button>
                  </div>
                </div>
              )}
              {msg.proposalStatus === "confirmed" && (
                <div className="mt-2 pt-2 border-t border-[var(--shell-input-border)]">
                  <span className="text-[10px] font-medium text-[var(--success)]">✓ Dikonfirmasi</span>
                </div>
              )}
              {msg.proposalStatus === "cancelled" && (
                <div className="mt-2 pt-2 border-t border-[var(--shell-input-border)]">
                  <span className="text-[10px] font-medium text-[var(--shell-muted)]">✗ Dibatalkan</span>
                </div>
              )}
            </div>
          </div>
        ))}

        {isSending && (
          <div className="flex justify-start">
            <div className="inline-flex items-center gap-3 rounded-2xl px-4 py-3">
              <div className="flex items-center gap-1 h-2">
                <span className="h-1 w-1 rounded-full bg-[var(--brand)] animate-bounce [animation-delay:0ms]" />
                <span className="h-1 w-1 rounded-full bg-[var(--brand)] animate-bounce [animation-delay:150ms]" />
                <span className="h-1 w-1 rounded-full bg-[var(--brand)] animate-bounce [animation-delay:300ms]" />
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Error */}
      {error && (
        <div className="mx-4 mb-2 rounded-lg bg-[var(--danger-soft)] border border-[var(--danger)]/20 px-3 py-2 text-[10px] text-[var(--danger)]">
          {error}
        </div>
      )}

      {/* Input area */}
      <form onSubmit={handleSubmit} className="shrink-0 p-3 pt-0">
        <div className="rounded-xl bg-[var(--shell-input-bg)] border border-[var(--shell-input-border)] ring-0">
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

// parseActiveEntities walks the current pathname and extracts the
// entity IDs the user is looking at. The backend uses these to enrich
// the AI system prompt with concrete state (e.g. when the user is on
// /app/exams/{id} the AI sees the exam title, section + question
// count, blueprint kisi-kisi slots) so suggestions stay grounded in
// the page and don't duplicate existing items.
//
// Recognised shapes (UUID validated to avoid leaking junk):
//   /app/exams/{examId}              → { view: 'exam-detail',     examId }
//   /app/blueprints/{templateId}     → { view: 'blueprint-detail', templateId }
//   /app/courses/{courseId}          → { view: 'course-detail',   courseId }
//   /app/stimuli/{stimulusId}        → { view: 'stimulus-detail', stimulusId }
// Anything else → { view: '<segment>' } so the model can still see
// where the user is.
function parseActiveEntities(pathname: string): Record<string, string> {
  if (!pathname) return {};
  const segments = pathname.split("/").filter(Boolean);
  // strip the leading 'app'
  const after = segments[0] === "app" ? segments.slice(1) : segments;
  if (after.length === 0) return { view: "home" };

  const entities: Record<string, string> = { view: after.join(":") };
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

  // Match shape /app/{collection}/{id}
  if (after.length >= 2 && uuid.test(after[1])) {
    const collection = after[0];
    const id = after[1];
    switch (collection) {
      case "exams":
        entities.view = "exam-detail";
        entities.examId = id;
        break;
      case "blueprints":
        entities.view = "blueprint-detail";
        entities.templateId = id;
        break;
      case "courses":
        entities.view = "course-detail";
        entities.courseId = id;
        break;
      case "stimuli":
        entities.view = "stimulus-detail";
        entities.stimulusId = id;
        break;
      case "programs":
        entities.view = "program-detail";
        entities.programId = id;
        break;
      default:
        entities.view = `${collection}-detail`;
        entities.resourceId = id;
    }
  } else {
    // List / static page — just record the path so the model knows
    // "the user is on /app/exams" and can ask for a specific item.
    entities.view = after.join(":");
  }
  return entities;
}
