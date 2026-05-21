"use client";

/**
 * StimulusPicker — inline picker reused by question accordions
 * (per-question stimulus axis) and group cards (group-shared stimulus).
 *
 * Two tabs:
 *   - Library: search shared stimuli, click to select
 *   - Tulis Baru: title + body markdown, save creates a new shared
 *                 stimulus and selects it
 *
 * The picker is pure UI: parent owns the selected stimulusId and
 * decides what to do with it (set on a question via update, set on a
 * group via createQuestionGroup / updateQuestionGroup).
 */

import { useEffect, useMemo, useState } from "react";
import { Loader2, Search, FileText, Sparkles, Check } from "lucide-react";
import {
  listStimuli,
  createStimulus,
  type Stimulus,
} from "@/lib/modules-api";
import { useToast } from "@/components/ui/toast";
import { InputField } from "@/components/ui/input-field";
import { RichEditor } from "@/components/ui/rich-editor";
import { RenderedContent } from "@/components/ui/rendered-content";
import { cn } from "@/lib/cn";

interface StimulusPickerProps {
  /** Currently selected stimulusId, if any. */
  value: string | null;
  /** Called when user picks an existing or newly-created stimulus. */
  onSelect: (stimulus: Stimulus) => void;
  /** Called when user clears selection. */
  onClear?: () => void;
  /** Optional title shown at the top. */
  title?: string;
}

export function StimulusPicker({
  value,
  onSelect,
  onClear,
  title,
}: StimulusPickerProps) {
  const { toast } = useToast();
  // Default to inline 'Tulis baru' tab. Stimulus library is now an
  // opt-in power-user feature (Opsi B): most teachers want a fresh
  // stimulus per group rather than reusing one from a shared pool.
  // Library tab still works for the cases that genuinely need it.
  const [tab, setTab] = useState<"library" | "new">("new");
  const [search, setSearch] = useState("");
  const [items, setItems] = useState<Stimulus[]>([]);
  const [loading, setLoading] = useState(false);

  const [newTitle, setNewTitle] = useState("");
  const [newBody, setNewBody] = useState("");
  const [creating, setCreating] = useState(false);

  // Debounced search. Lightweight — refresh on tab open + on search.
  useEffect(() => {
    if (tab !== "library") return;
    let cancelled = false;
    const t = setTimeout(async () => {
      setLoading(true);
      const res = await listStimuli({
        lifecycle: "shared",
        search: search || undefined,
      });
      if (!cancelled) {
        if (res.data) setItems(res.data.data);
        setLoading(false);
      }
    }, 200);
    return () => {
      cancelled = true;
      clearTimeout(t);
    };
  }, [search, tab]);

  const filtered = useMemo(() => items, [items]);

  // The rich editor reports an "empty" document as an empty <p></p>
  // string. Strip tags + whitespace so the disabled-button check
  // doesn't let a user submit a blank stimulus that just happens to
  // contain markup.
  const newBodyHasText =
    newBody.replace(/<[^>]+>/g, "").replace(/\s+/g, "").length > 0;

  async function handleCreate() {
    if (!newTitle.trim() || !newBodyHasText) {
      toast({
        tone: "error",
        title: "Lengkapi judul dan isi",
        description: "Stimulus butuh judul dan isi sebelum disimpan.",
      });
      return;
    }
    setCreating(true);
    const res = await createStimulus({
      title: newTitle.trim(),
      content: newBody,
      lifecycle: "shared",
    });
    setCreating(false);
    if (res.error || !res.data) {
      toast({
        tone: "error",
        title: "Gagal menyimpan",
        description: res.error?.message ?? "Tidak diketahui",
      });
      return;
    }
    // Re-fetch so the new entry surfaces in the library tab too.
    const list = await listStimuli({ lifecycle: "shared" });
    if (list.data) setItems(list.data.data);
    const created = list.data?.data.find((s) => s.id === res.data!.id);
    if (created) {
      onSelect(created);
    }
    setNewTitle("");
    setNewBody("");
    setTab("new");
    toast({ tone: "success", title: "Stimulus disimpan" });
  }

  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--card)]">
      {title && (
        <div className="border-b border-[var(--border)] bg-[var(--accent)]/40 px-3 py-2 text-[11px] font-semibold text-[var(--foreground)]">
          {title}
        </div>
      )}
      {/* Tabs */}
      <div className="flex items-center gap-1 border-b border-[var(--border)] bg-[var(--accent)]/30 px-2 py-1.5">
        <TabButton
          active={tab === "new"}
          onClick={() => setTab("new")}
          icon={<Sparkles size={11} />}
          label="Tulis baru"
        />
        <TabButton
          active={tab === "library"}
          onClick={() => setTab("library")}
          icon={<Search size={11} />}
          label="Dari library"
        />
        {value && onClear && (
          <button
            type="button"
            onClick={onClear}
            className="ml-auto rounded-md px-2 py-1 text-[10.5px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--destructive)] transition-colors"
          >
            Hapus stimulus
          </button>
        )}
      </div>

      {tab === "library" ? (
        <div className="p-3 space-y-2">
          <div className="relative">
            <Search
              size={13}
              className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--muted-foreground)]"
            />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-8 w-full rounded-md border border-[var(--border)] bg-[var(--background)] pl-7 pr-2 text-[12px] text-[var(--foreground)] outline-none focus:border-[var(--brand)] focus:ring-2 focus:ring-[var(--field-ring)]"
              aria-label="Cari stimulus"
            />
            {!search && (
              <span className="pointer-events-none absolute left-7 top-1/2 -translate-y-1/2 text-[12px] text-[var(--muted-foreground)]">
                Cari stimulus...
              </span>
            )}
          </div>

          {loading ? (
            <div className="flex items-center justify-center py-6 text-[var(--muted-foreground)]">
              <Loader2 size={14} className="animate-spin" />
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-1 py-5 text-center">
              <FileText
                size={20}
                className="text-[var(--muted-foreground)]"
              />
              <p className="text-[11px] text-[var(--muted-foreground)]">
                {search
                  ? "Tidak ada hasil. Coba kata kunci lain atau tulis baru."
                  : "Belum ada stimulus shared. Tulis baru di tab sebelah."}
              </p>
            </div>
          ) : (
            <div className="max-h-56 overflow-y-auto rounded-md border border-[var(--border)] bg-[var(--background)]">
              {filtered.map((s) => (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => onSelect(s)}
                  className={cn(
                    "flex w-full items-start gap-2 border-b border-[var(--border)] px-2.5 py-2 text-left transition-colors last:border-b-0 hover:bg-[var(--muted)]",
                    value === s.id && "bg-[var(--brand-soft)]",
                  )}
                >
                  <div className="flex-1 min-w-0">
                    <p className="text-[12px] font-medium text-[var(--foreground)] truncate">
                      {s.title}
                    </p>
                    <div className="mt-0.5 line-clamp-2 text-[10.5px] text-[var(--muted-foreground)]">
                      <RenderedContent
                        html={s.content}
                        className="text-[10.5px] [&_p]:my-0"
                      />
                    </div>
                  </div>
                  {value === s.id && (
                    <Check
                      size={12}
                      className="mt-0.5 shrink-0 text-[var(--brand)]"
                    />
                  )}
                </button>
              ))}
            </div>
          )}
        </div>
      ) : (
        <div className="p-3 space-y-2">
          <InputField
            label="Judul stimulus"
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
          />
          <div>
            <label className="mb-1 block text-[11px] font-medium text-[var(--muted-foreground)]">
              Isi stimulus
            </label>
            <RichEditor
              value={newBody}
              onChange={setNewBody}
              minRows={5}
              placeholder="Bacaan, kasus, atau materi pendukung. LaTeX didukung."
              ariaLabel="Isi stimulus"
            />
          </div>
          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={() => {
                setNewTitle("");
                setNewBody("");
              }}
              disabled={creating}
              className="h-8 rounded-md px-2.5 text-[11px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] disabled:opacity-50 transition-colors"
            >
              Reset
            </button>
            <button
              type="button"
              onClick={handleCreate}
              disabled={creating || !newTitle.trim() || !newBodyHasText}
              className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[var(--primary)] px-2.5 text-[11px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 disabled:opacity-50 transition-all"
            >
              {creating && <Loader2 size={11} className="animate-spin" />}
              Simpan & pilih
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function TabButton({
  active,
  onClick,
  icon,
  label,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "inline-flex h-7 items-center gap-1 rounded-md px-2 text-[11px] font-medium transition-colors",
        active
          ? "bg-[var(--card)] text-[var(--foreground)] shadow-sm"
          : "text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]",
      )}
    >
      {icon}
      {label}
    </button>
  );
}
