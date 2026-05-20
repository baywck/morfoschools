"use client";

import { useEffect, useState, use } from "react";
import { useRouter } from "next/navigation";
import {
  getBlueprintTemplate,
  listTemplateSlots,
  createTemplateSlot,
  updateTemplateSlot,
  deleteTemplateSlot,
  publishBlueprintTemplate,
  unpublishBlueprintTemplate,
  archiveBlueprintTemplate,
  type BlueprintTemplate,
  type BlueprintSlot,
  type SlotPayload,
} from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { ShareDialog } from "@/components/share-dialog";
import {
  Plus,
  Pencil,
  Trash2,
  Send,
  Share2,
  Crown,
  Lock,
} from "lucide-react";
import { cn } from "@/lib/cn";
import { KisiKisiFields } from "@/components/blueprint/kisi-kisi-fields";

const QUESTION_TYPES = [
  { value: "multiple_choice", label: "Pilihan Ganda" },
  { value: "true_false", label: "Benar/Salah" },
  { value: "short_answer", label: "Isian Singkat" },
  { value: "essay", label: "Essay" },
];

const emptySlotForm: SlotPayload & { points: number } = {
  competencyCode: "",
  competencyDescription: "",
  materi: "",
  indikator: "",
  cognitiveLevel: "",
  difficulty: "",
  questionType: "multiple_choice",
  points: 1,
  akmKonten: "",
  akmKonteks: "",
  akmProses: "",
  akmLevel: undefined,
};

export default function BlueprintDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id: templateId } = use(params);
  const router = useRouter();
  const { toast } = useToast();

  const [template, setTemplate] = useState<BlueprintTemplate | null>(null);
  const [slots, setSlots] = useState<BlueprintSlot[]>([]);
  const [loading, setLoading] = useState(true);

  // Slot edit/create state
  const [slotForm, setSlotForm] = useState<typeof emptySlotForm>(emptySlotForm);
  const [editingSlot, setEditingSlot] = useState<BlueprintSlot | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [submittingSlot, setSubmittingSlot] = useState(false);
  const [slotErrors, setSlotErrors] = useState<Record<string, string>>({});

  // Delete state
  const [deleteTarget, setDeleteTarget] = useState<BlueprintSlot | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Share + publish + archive
  const [showShare, setShowShare] = useState(false);
  const [confirmPublish, setConfirmPublish] = useState(false);
  const [confirmUnpublish, setConfirmUnpublish] = useState(false);
  const [working, setWorking] = useState(false);

  async function reload() {
    setLoading(true);
    const [tplRes, slotsRes] = await Promise.all([
      getBlueprintTemplate(templateId),
      listTemplateSlots(templateId),
    ]);
    if (tplRes.data) setTemplate(tplRes.data);
    if (slotsRes.data) setSlots(slotsRes.data.data);
    setLoading(false);
  }

  useEffect(() => {
    reload();
  }, [templateId]);

  const isAKM = template?.blueprintType?.startsWith("akm_");
  const isLocked = template?.status !== "draft";
  const canEdit = !!template && template.status === "draft";

  function openCreate() {
    setEditingSlot(null);
    setSlotForm({
      ...emptySlotForm,
      questionType: "multiple_choice",
    });
    setSlotErrors({});
    setShowCreate(true);
  }

  function openEdit(s: BlueprintSlot) {
    setEditingSlot(s);
    setSlotForm({
      competencyCode: s.competencyCode ?? "",
      competencyDescription: s.competencyDescription ?? "",
      materi: s.materi ?? "",
      indikator: s.indikator ?? "",
      cognitiveLevel: s.cognitiveLevel ?? "",
      difficulty: s.difficulty ?? "",
      questionType: s.questionType ?? "multiple_choice",
      points: s.points,
      akmKonten: s.akmKonten ?? "",
      akmKonteks: s.akmKonteks ?? "",
      akmProses: s.akmProses ?? "",
      akmLevel: s.akmLevel ?? undefined,
    });
    setSlotErrors({});
    setShowCreate(true);
  }

  async function submitSlot(e: React.FormEvent) {
    e.preventDefault();
    setSlotErrors({});
    setSubmittingSlot(true);
    const payload: SlotPayload = {
      competencyCode: slotForm.competencyCode || undefined,
      competencyDescription: slotForm.competencyDescription || undefined,
      materi: slotForm.materi || undefined,
      indikator: slotForm.indikator || undefined,
      cognitiveLevel: slotForm.cognitiveLevel || undefined,
      difficulty: slotForm.difficulty || undefined,
      questionType: slotForm.questionType || undefined,
      points: typeof slotForm.points === "number" ? slotForm.points : 1,
      akmKonten: slotForm.akmKonten || undefined,
      akmKonteks: slotForm.akmKonteks || undefined,
      akmProses: slotForm.akmProses || undefined,
      akmLevel: slotForm.akmLevel,
    };
    const res = editingSlot
      ? await updateTemplateSlot(editingSlot.id, payload)
      : await createTemplateSlot(templateId, payload);
    setSubmittingSlot(false);
    if (res.error) {
      if (res.error.fields) setSlotErrors(res.error.fields);
      else
        toast({ tone: "error", title: "Save failed", description: res.error.message });
      return;
    }
    toast({
      tone: "success",
      title: editingSlot ? "Slot updated" : "Slot added",
    });
    setShowCreate(false);
    reload();
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    const res = await deleteTemplateSlot(deleteTarget.id);
    setDeleting(false);
    if (res.error) {
      toast({ tone: "error", title: "Delete failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Slot deleted" });
    setDeleteTarget(null);
    reload();
  }

  async function handlePublish() {
    if (!template) return;
    if (slots.length === 0) {
      toast({
        tone: "error",
        title: "Tidak bisa publish",
        description: "Tambah minimal satu slot dulu.",
      });
      return;
    }
    setWorking(true);
    const res = await publishBlueprintTemplate(template.id);
    setWorking(false);
    setConfirmPublish(false);
    if (res.error) {
      toast({ tone: "error", title: "Publish failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Blueprint published" });
    reload();
  }

  async function handleUnpublish() {
    if (!template) return;
    setWorking(true);
    const res = await unpublishBlueprintTemplate(template.id);
    setWorking(false);
    setConfirmUnpublish(false);
    if (res.error) {
      toast({
        tone: "error",
        title: "Set to draft failed",
        description: res.error.message,
      });
      return;
    }
    toast({
      tone: "success",
      title: "Template kembali ke draft",
      description: "Slot bisa diedit lagi sekarang.",
    });
    reload();
  }

  async function handleArchive() {
    if (!template) return;
    setWorking(true);
    const res = await archiveBlueprintTemplate(template.id);
    setWorking(false);
    if (res.error) {
      toast({ tone: "error", title: "Archive failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: "Blueprint archived" });
    router.push("/app/blueprints");
  }

  if (loading) {
    return (
      <PageShell title="Blueprint" subtitle="Loading...">
        <div className="space-y-3">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      </PageShell>
    );
  }

  if (!template) {
    return (
      <PageShell title="Blueprint" subtitle="">
        <div className="rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
          <p className="text-[13px] font-semibold text-[var(--foreground)]">
            Blueprint tidak ditemukan
          </p>
          <button
            type="button"
            onClick={() => router.push("/app/blueprints")}
            className="mt-3 inline-flex h-8 items-center rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] transition-all"
          >
            Kembali ke list
          </button>
        </div>
      </PageShell>
    );
  }

  return (
    <>
      <PageShell
        title={template.title}
        subtitle={`${template.curriculumCode.toUpperCase()} · ${template.blueprintType.replace("akm_", "AKM ")} · ${slots.length} slot${slots.length !== 1 ? "s" : ""} · ${template.totalPoints} pts`}
        back={{ href: "/app/blueprints", label: "Back to blueprints" }}
        actions={
          <>
            <button
              type="button"
              onClick={() => setShowShare(true)}
              className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-[12px] font-medium text-[var(--foreground)] hover:bg-[var(--muted)] transition-colors"
            >
              <Share2 size={14} /> Collaborator
            </button>
            {canEdit ? (
              <>
                <button
                  type="button"
                  onClick={openCreate}
                  className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] transition-all"
                >
                  <Plus size={14} /> Add Slot
                </button>
                {template.status === "draft" && (
                  <button
                    type="button"
                    onClick={() => setConfirmPublish(true)}
                    disabled={slots.length === 0}
                    className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[var(--brand)] bg-[var(--brand-soft)] px-3 text-[12px] font-semibold text-[var(--brand)] hover:bg-[var(--brand)] hover:text-white disabled:opacity-60 disabled:cursor-not-allowed transition-all"
                  >
                    <Send size={14} /> Publish
                  </button>
                )}
              </>
            ) : (
              template.status === "published" && (
                <button
                  type="button"
                  onClick={() => setConfirmUnpublish(true)}
                  className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-[12px] font-medium text-[var(--foreground)] hover:bg-[var(--muted)] transition-colors"
                  title="Set ke draft agar bisa diedit lagi"
                >
                  <Pencil size={14} /> Set to draft
                </button>
              )
            )}
          </>
        }
      >
        {/* Status + meta strip */}
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "rounded-md px-2 py-0.5 text-[10px] font-medium",
              template.status === "published"
                ? "bg-[var(--success-soft)] text-[var(--success)]"
                : template.status === "archived"
                  ? "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  : "bg-[var(--warning-soft)] text-[var(--warning)]",
            )}
          >
            {template.status}
          </span>
          {template.strictCoverage && (
            <span className="rounded-md bg-[var(--brand-soft)] px-2 py-0.5 text-[10px] font-medium text-[var(--brand)]">
              strict coverage
            </span>
          )}
          {template.competencyLabel && (
            <span className="rounded-md bg-[var(--accent)] px-2 py-0.5 text-[10px] font-medium text-[var(--muted-foreground)]">
              {template.competencyLabel}
            </span>
          )}
        </div>

        {template.description && (
          <p className="mb-4 rounded-lg border border-[var(--border)] bg-[var(--card)] p-3 text-[12px] text-[var(--muted-foreground)]">
            {template.description}
          </p>
        )}

        {isLocked && (
          <div className="mb-4 flex items-start gap-2 rounded-lg border border-[var(--border)] bg-[var(--accent)] p-3">
            <Lock size={14} className="text-[var(--muted-foreground)] mt-0.5" />
            <p className="text-[11px] text-[var(--muted-foreground)]">
              Template {template.status}. Slot tidak bisa di-edit.{" "}
              {template.status === "published" &&
                "Untuk mengubah, archive lalu buat versi baru."}
            </p>
          </div>
        )}

        {/* Slots table */}
        {slots.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <Crown size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">
              Belum ada slot
            </p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">
              Tambah slot untuk mulai membangun struktur kisi-kisi.
            </p>
            {canEdit && (
              <button
                type="button"
                onClick={openCreate}
                className="mt-4 inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] transition-all"
              >
                <Plus size={12} /> Tambah slot pertama
              </button>
            )}
          </div>
        ) : (
          <div className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)]">
            <div className="overflow-x-auto">
              <table className="w-full text-[12px]">
                <thead className="bg-[var(--accent)] text-[10px] uppercase tracking-wide text-[var(--muted-foreground)]">
                  <tr>
                    <th className="w-10 px-3 py-2 text-left font-medium">#</th>
                    <th className="px-3 py-2 text-left font-medium">{template.competencyLabel || "Kompetensi"}</th>
                    <th className="px-3 py-2 text-left font-medium">Materi</th>
                    <th className="px-3 py-2 text-left font-medium">Level</th>
                    <th className="px-3 py-2 text-left font-medium">Tingkat</th>
                    <th className="px-3 py-2 text-left font-medium">Tipe</th>
                    {isAKM && <th className="px-3 py-2 text-left font-medium">AKM</th>}
                    <th className="w-16 px-3 py-2 text-right font-medium">Pts</th>
                    {canEdit && <th className="w-20 px-3 py-2 text-right font-medium">Action</th>}
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border)]">
                  {slots.map((s) => (
                    <tr
                      key={s.id}
                      className={cn(
                        "transition-colors",
                        canEdit ? "hover:bg-[var(--muted)]/30 cursor-pointer" : "",
                      )}
                      onClick={() => canEdit && openEdit(s)}
                    >
                      <td className="px-3 py-2.5 text-[var(--muted-foreground)]">
                        {s.position + 1}
                      </td>
                      <td className="px-3 py-2.5 font-medium text-[var(--foreground)]">
                        {s.competencyCode || (
                          <span className="text-[var(--muted-foreground)] italic">—</span>
                        )}
                        {s.competencyDescription && (
                          <p className="mt-0.5 text-[10px] font-normal text-[var(--muted-foreground)] line-clamp-1">
                            {s.competencyDescription}
                          </p>
                        )}
                      </td>
                      <td className="px-3 py-2.5 text-[var(--foreground)]">
                        {s.materi || (
                          <span className="text-[var(--muted-foreground)] italic">—</span>
                        )}
                      </td>
                      <td className="px-3 py-2.5">
                        {s.cognitiveLevel ? (
                          <span className="rounded-md bg-[var(--brand-soft)] px-1.5 py-0.5 text-[10px] font-semibold text-[var(--brand)]">
                            {s.cognitiveLevel}
                          </span>
                        ) : (
                          <span className="text-[var(--muted-foreground)] italic">—</span>
                        )}
                      </td>
                      <td className="px-3 py-2.5 text-[var(--muted-foreground)] capitalize">
                        {s.difficulty || "—"}
                      </td>
                      <td className="px-3 py-2.5 text-[var(--muted-foreground)] text-[10px]">
                        {s.questionType
                          ? QUESTION_TYPES.find((q) => q.value === s.questionType)?.label ||
                            s.questionType
                          : "—"}
                      </td>
                      {isAKM && (
                        <td className="px-3 py-2.5 text-[10px] text-[var(--muted-foreground)]">
                          {[s.akmKonten, s.akmKonteks, s.akmProses, s.akmLevel ? `L${s.akmLevel}` : ""]
                            .filter(Boolean)
                            .join(" · ") || "—"}
                        </td>
                      )}
                      <td className="px-3 py-2.5 text-right font-mono text-[var(--foreground)]">
                        {s.points}
                      </td>
                      {canEdit && (
                        <td className="px-3 py-2.5 text-right" onClick={(e) => e.stopPropagation()}>
                          <div className="flex items-center justify-end gap-1">
                            <button
                              type="button"
                              onClick={() => openEdit(s)}
                              className="flex h-6 w-6 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]"
                            >
                              <Pencil size={11} />
                            </button>
                            <button
                              type="button"
                              onClick={() => setDeleteTarget(s)}
                              className="flex h-6 w-6 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--danger-soft)] hover:text-[var(--danger)]"
                            >
                              <Trash2 size={11} />
                            </button>
                          </div>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
                <tfoot className="bg-[var(--accent)] text-[11px]">
                  <tr>
                    <td colSpan={isAKM ? 7 : 6} className="px-3 py-2 text-right font-medium text-[var(--muted-foreground)]">
                      Total
                    </td>
                    <td className="px-3 py-2 text-right font-mono font-semibold text-[var(--foreground)]">
                      {template.totalPoints}
                    </td>
                    {canEdit && <td />}
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>
        )}

        {canEdit && template.status === "draft" && slots.length > 0 && (
          <button
            type="button"
            onClick={handleArchive}
            className="mt-6 text-[11px] text-[var(--danger)] hover:underline"
          >
            Archive blueprint
          </button>
        )}
      </PageShell>

      {/* Slot create/edit sheet */}
      <RightPullSheet
        open={showCreate}
        title={editingSlot ? `Edit Slot #${editingSlot.position + 1}` : "Add Slot"}
        onClose={() => setShowCreate(false)}
      >
        <form onSubmit={submitSlot} className="space-y-3">
          {/* Shared kisi-kisi block (Phase 9.10) — same form is used
              by the inline question accordion editor. competencyDescription
              + questionType + points stay outside this block since they
              are blueprint-only metadata, not part of the kisi-kisi axis. */}
          <KisiKisiFields
            isAkm={!!isAKM}
            competency={slotForm.competencyCode ?? ""}
            competencyDescription={slotForm.competencyDescription ?? ""}
            materi={slotForm.materi ?? ""}
            indikator={slotForm.indikator ?? ""}
            cognitive={slotForm.cognitiveLevel ?? ""}
            difficulty={slotForm.difficulty ?? ""}
            akmKonten={slotForm.akmKonten ?? ""}
            akmKonteks={slotForm.akmKonteks ?? ""}
            akmProses={slotForm.akmProses ?? ""}
            akmLevel={
              slotForm.akmLevel != null ? String(slotForm.akmLevel) : ""
            }
            onCompetency={(v) =>
              setSlotForm({ ...slotForm, competencyCode: v })
            }
            onCompetencyDescription={(v) =>
              setSlotForm({ ...slotForm, competencyDescription: v })
            }
            onMateri={(v) => setSlotForm({ ...slotForm, materi: v })}
            onIndikator={(v) => setSlotForm({ ...slotForm, indikator: v })}
            onCognitive={(v) =>
              setSlotForm({ ...slotForm, cognitiveLevel: v })
            }
            onDifficulty={(v) =>
              setSlotForm({ ...slotForm, difficulty: v })
            }
            onAkmKonten={(v) => setSlotForm({ ...slotForm, akmKonten: v })}
            onAkmKonteks={(v) =>
              setSlotForm({ ...slotForm, akmKonteks: v })
            }
            onAkmProses={(v) => setSlotForm({ ...slotForm, akmProses: v })}
            onAkmLevel={(v) =>
              setSlotForm({
                ...slotForm,
                akmLevel: v ? Number(v) : undefined,
              })
            }
          />
          <div className="grid grid-cols-2 gap-2">
            <SelectField
              label="Tipe Soal"
              value={slotForm.questionType ?? "multiple_choice"}
              onChange={(v) => setSlotForm({ ...slotForm, questionType: v })}
              options={QUESTION_TYPES}
            />
            <InputField
              label="Points"
              value={String(slotForm.points)}
              onChange={(e) => {
                const n = Number(e.target.value);
                setSlotForm({
                  ...slotForm,
                  points: Number.isFinite(n) && n >= 0 ? n : 0,
                });
              }}
              error={slotErrors.points}
            />
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submittingSlot}
              className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all"
            >
              {submittingSlot && (
                <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />
              )}
              {editingSlot ? "Update slot" : "Add slot"}
            </button>
          </div>
        </form>
      </RightPullSheet>

      <ShareDialog
        open={showShare}
        onClose={() => setShowShare(false)}
        resource="blueprint-templates"
        resourceId={template.id}
        resourceName={template.title}
        currentUserCanManage={template.canAccess}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete slot?"
        description={`Slot #${deleteTarget ? deleteTarget.position + 1 : ""} akan dihapus permanen.`}
        confirmLabel="Delete"
        destructive
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />

      <ConfirmDialog
        open={confirmPublish}
        title="Publish blueprint?"
        description={`Setelah publish, slot tidak bisa di-edit. Untuk perubahan, archive lalu buat versi baru.`}
        confirmLabel="Publish"
        loading={working}
        onConfirm={handlePublish}
        onCancel={() => setConfirmPublish(false)}
      />

      <ConfirmDialog
        open={confirmUnpublish}
        title="Set blueprint ke draft?"
        description="Template akan kembali ke status draft sehingga slot bisa diedit lagi. Exam yang sudah pernah meng-clone blueprint ini tidak terpengaruh."
        confirmLabel="Set to draft"
        loading={working}
        onConfirm={handleUnpublish}
        onCancel={() => setConfirmUnpublish(false)}
      />
    </>
  );
}
