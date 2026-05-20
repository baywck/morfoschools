"use client";

/**
 * ExportBlueprintSheet (Phase 9.10)
 *
 * Exports the exam's current kisi-kisi (exam_blueprint + slots) as a
 * brand-new draft blueprint_template. The teacher iterates per-question
 * inline on the exam canvas — when the structure feels right, they hit
 * Export to lift the blueprint into the library for reuse next term.
 *
 * The new template is created in 'draft' status and owned by the
 * caller; subject/grade are inherited from the source blueprint but
 * overridable via the form. Slot stimulus FKs are preserved.
 */

import { useState } from "react";
import { useRouter } from "next/navigation";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { useToast } from "@/components/ui/toast";
import { exportExamBlueprintToTemplate } from "@/lib/modules-api";
import { Loader2, ClipboardCopy } from "lucide-react";
import { cn } from "@/lib/cn";

export interface ExportBlueprintSheetProps {
  open: boolean;
  examId: string;
  defaultTitle: string;
  onClose: () => void;
}

export function ExportBlueprintSheet({
  open,
  examId,
  defaultTitle,
  onClose,
}: ExportBlueprintSheetProps) {
  const router = useRouter();
  const { toast } = useToast();
  const [title, setTitle] = useState(defaultTitle);
  const [description, setDescription] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    if (!title.trim()) {
      setErrors({ title: "Judul wajib diisi" });
      return;
    }
    setSubmitting(true);
    // Subject + grade are inherited from the source exam blueprint by
    // the backend when omitted — there is no UX value in re-prompting,
    // they always match the exam being exported.
    const res = await exportExamBlueprintToTemplate(examId, {
      title: title.trim(),
      description: description.trim() || undefined,
    });
    setSubmitting(false);
    if (res.error) {
      if (res.error.fields) {
        setErrors(res.error.fields);
      }
      toast({
        tone: "error",
        title: "Export gagal",
        description: res.error.message,
      });
      return;
    }
    toast({
      tone: "success",
      title: "Kisi-kisi diexport",
      description: "Template baru siap diedit di library.",
    });
    onClose();
    if (res.data?.id) {
      router.push(`/app/blueprints/${res.data.id}`);
    }
  }

  return (
    <RightPullSheet
      open={open}
      title="Export kisi-kisi sebagai template"
      onClose={onClose}
    >
      <form onSubmit={submit} className="space-y-3">
        <div className="rounded-lg border border-[var(--border)] bg-[var(--accent)] p-3 text-[11px] leading-relaxed text-[var(--muted-foreground)]">
          <ClipboardCopy
            size={12}
            className="mr-1 inline align-[-2px] text-[var(--brand)]"
          />
          Template baru akan dibuat <strong>draft</strong> dan dimiliki
          oleh kamu. Subject, grade, dan slot mengikuti exam ini.
        </div>
        <InputField
          label="Judul template"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          error={errors.title}
        />
        <InputField
          label="Deskripsi (opsional)"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <div className="flex justify-end gap-2 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
          >
            Batal
          </button>
          <button
            type="submit"
            disabled={submitting}
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm transition-all",
              "hover:opacity-90 active:scale-[0.97]",
              submitting && "opacity-60 cursor-not-allowed",
            )}
          >
            {submitting && <Loader2 size={12} className="animate-spin" />}
            Export
          </button>
        </div>
      </form>
    </RightPullSheet>
  );
}
