"use client";

import { useEffect, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { BookOpen } from "lucide-react";
import { cn } from "@/lib/cn";
import {
  listPrograms,
  createProgram,
  updateProgram,
  archiveProgram,
  publishProgram,
  type Program,
} from "@/lib/modules-api";

export default function ProgramsPage() {
  const { toast } = useToast();
  const [data, setData] = useState<Program[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [search, setSearch] = useState("");

  const [createOpen, setCreateOpen] = useState(false);
  const [editItem, setEditItem] = useState<Program | null>(null);
  const [archiveItem, setArchiveItem] = useState<Program | null>(null);

  // Form states
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [kind, setKind] = useState("regular");
  const [gradeLevel, setGradeLevel] = useState("");

  const fetchData = async () => {
    try {
      setLoading(true);
      const res = await listPrograms({ search });
      if (res.data) setData(res.data.data);
    } catch (error: any) {
      toast({
        title: "Failed to load programs",
        description: error.message,
        tone: "error",
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const debounce = setTimeout(fetchData, 300);
    return () => clearTimeout(debounce);
  }, [search]);

  const resetForm = () => {
    setTitle("");
    setDescription("");
    setKind("regular");
    setGradeLevel("");
    setEditItem(null);
  };

  const handleCreate = async () => {
    if (!title.trim()) return;
    try {
      setSaving(true);
      await createProgram({
        title,
        description,
        kind,
        gradeLevel,
      });
      toast({ title: "Program created", tone: "success" });
      setCreateOpen(false);
      resetForm();
      fetchData();
    } catch (error: any) {
      toast({
        title: "Failed to create program",
        description: error.message,
        tone: "error",
      });
    } finally {
      setSaving(false);
    }
  };

  const handleUpdate = async () => {
    if (!editItem || !title.trim()) return;
    try {
      setSaving(true);
      await updateProgram(editItem.id, {
        title,
        description,
        kind,
        gradeLevel,
      });
      toast({ title: "Program updated", tone: "success" });
      setEditItem(null);
      resetForm();
      fetchData();
    } catch (error: any) {
      toast({
        title: "Failed to update program",
        description: error.message,
        tone: "error",
      });
    } finally {
      setSaving(false);
    }
  };

  const handleArchive = async () => {
    if (!archiveItem) return;
    try {
      setSaving(true);
      await archiveProgram(archiveItem.id);
      toast({ title: "Program archived", tone: "success" });
      setArchiveItem(null);
      fetchData();
    } catch (error: any) {
      toast({
        title: "Failed to archive program",
        description: error.message,
        tone: "error",
      });
    } finally {
      setSaving(false);
    }
  };

  const handlePublish = async (id: string) => {
    try {
      setSaving(true);
      await publishProgram(id);
      toast({ title: "Program published", tone: "success" });
      fetchData();
    } catch (error: any) {
      toast({
        title: "Failed to publish program",
        description: error.message,
        tone: "error",
      });
    } finally {
      setSaving(false);
    }
  };

  const openEdit = (item: Program) => {
    setEditItem(item);
    setTitle(item.title);
    setDescription(item.description || "");
    setKind(item.kind);
    setGradeLevel(item.gradeLevel || "");
  };

  return (
    <>
      <PageShell
        title="Programs"
        subtitle="Manage learning programs and enrollments"
        search={{ value: search, onChange: setSearch }}
        onAdd={() => {
          resetForm();
          setCreateOpen(true);
        }}
        addLabel="New Program"
      >
        <div className="flex flex-col gap-2">
          {loading ? (
            Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 p-3 bg-white rounded-lg border border-[var(--border)] shadow-sm">
                <Skeleton className="h-8 w-8 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-48" />
                  <Skeleton className="h-3 w-24" />
                </div>
              </div>
            ))
          ) : data.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-8 text-center bg-white rounded-lg border border-dashed border-[var(--border)]">
              <BookOpen className="h-8 w-8 text-[var(--muted)] mb-3" />
              <p className="text-sm font-medium text-[var(--foreground)]">No programs found</p>
              <p className="text-xs text-[var(--muted)] mt-1">Try adjusting your search or create a new program.</p>
            </div>
          ) : (
            data.map((item) => (
              <div
                key={item.id}
                className={cn(
                  "flex items-center gap-3 px-3 py-3 bg-white rounded-lg border border-[var(--border)] shadow-sm transition-colors hover:bg-[var(--hover)]",
                  item.status === "archived" && "opacity-60 grayscale"
                )}
              >
                <div className="h-8 w-8 flex items-center justify-center rounded-full bg-[var(--primary)]/10 text-[var(--primary)] shrink-0">
                  <BookOpen className="h-4 w-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium text-[var(--foreground)] truncate">
                      {item.title}
                    </p>
                    <span className="rounded-md px-2 py-0.5 text-[10px] font-medium bg-[var(--primary)]/10 text-[var(--primary)] capitalize">
                      {item.kind}
                    </span>
                    <span
                      className={cn(
                        "rounded-md px-2 py-0.5 text-[10px] font-medium capitalize",
                        item.status === "published"
                          ? "bg-emerald-100 text-emerald-700"
                          : item.status === "archived"
                          ? "bg-gray-100 text-gray-600"
                          : "bg-amber-100 text-amber-700"
                      )}
                    >
                      {item.status}
                    </span>
                  </div>
                  {item.description && (
                    <p className="text-xs text-[var(--muted)] truncate mt-0.5">
                      {item.description}
                    </p>
                  )}
                </div>
                <RowActions
                  actions={[
                    {
                      label: "Edit",
                      onClick: () => openEdit(item),
                    },
                    ...(item.status === "draft"
                      ? [
                          {
                            label: "Publish",
                            onClick: () => handlePublish(item.id),
                          },
                        ]
                      : []),
                    ...(item.status !== "archived"
                      ? [
                          {
                            label: "Archive",
                            onClick: () => setArchiveItem(item),
                            variant: "danger" as const,
                          },
                        ]
                      : []),
                  ]}
                />
              </div>
            ))
          )}
        </div>
      </PageShell>

      <RightPullSheet
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="Create Program"
      >
        <form onSubmit={(e) => { e.preventDefault(); handleCreate(); }} className="space-y-3">
          <InputField
            label="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <SelectField
            label="Kind"
            value={kind}
            onChange={(val) => setKind(val)}
            options={[
              { label: "Regular", value: "regular" },
              { label: "Remedial", value: "remedial" },
              { label: "Enrichment", value: "enrichment" },
              { label: "Tryout", value: "tryout" },
            ]}
          />
          <InputField
            label="Grade Level (Optional)"
            value={gradeLevel}
            onChange={(e) => setGradeLevel(e.target.value)}
          />
          <InputField
            label="Description (Optional)"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => setCreateOpen(false)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={saving || !title.trim()} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {saving && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      <RightPullSheet
        open={!!editItem}
        onClose={() => setEditItem(null)}
        title="Edit Program"
      >
        <form onSubmit={(e) => { e.preventDefault(); handleUpdate(); }} className="space-y-3">
          <InputField
            label="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <SelectField
            label="Kind"
            value={kind}
            onChange={(val) => setKind(val)}
            options={[
              { label: "Regular", value: "regular" },
              { label: "Remedial", value: "remedial" },
              { label: "Enrichment", value: "enrichment" },
              { label: "Tryout", value: "tryout" },
            ]}
          />
          <InputField
            label="Grade Level (Optional)"
            value={gradeLevel}
            onChange={(e) => setGradeLevel(e.target.value)}
          />
          <InputField
            label="Description (Optional)"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => setEditItem(null)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={saving || !title.trim()} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {saving ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : "Save Changes"}
            </button>
          </div>
        </form>
      </RightPullSheet>

      <ConfirmDialog
        open={!!archiveItem}
        onCancel={() => setArchiveItem(null)}
        title="Archive Program"
        description={`Are you sure you want to archive "${archiveItem?.title}"? It will no longer be visible to students.`}
        confirmLabel="Archive"
        onConfirm={handleArchive}
        loading={saving}
        destructive
      />
    </>
  );
}
