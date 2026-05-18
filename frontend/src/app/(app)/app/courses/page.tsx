"use client";

import { useEffect, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { FileText } from "lucide-react";
import { cn } from "@/lib/cn";
import {
  listCourses,
  createCourse,
  updateCourse,
  archiveCourse,
  publishCourse,
  type Course,
} from "@/lib/modules-api";

export default function CoursesPage() {
  const { toast } = useToast();
  const [data, setData] = useState<Course[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [search, setSearch] = useState("");

  const [createOpen, setCreateOpen] = useState(false);
  const [editItem, setEditItem] = useState<Course | null>(null);
  const [archiveItem, setArchiveItem] = useState<Course | null>(null);

  // Form states
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");

  const fetchData = async () => {
    try {
      setLoading(true);
      const res = await listCourses({ search });
      if (res.data) setData(res.data.data);
    } catch (error: any) {
      toast({
        title: "Failed to load courses",
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
    setEditItem(null);
  };

  const handleCreate = async () => {
    if (!title.trim()) return;
    try {
      setSaving(true);
      await createCourse({
        title,
        description,
      });
      toast({ title: "Course created", tone: "success" });
      setCreateOpen(false);
      resetForm();
      fetchData();
    } catch (error: any) {
      toast({
        title: "Failed to create course",
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
      await updateCourse(editItem.id, {
        title,
        description,
      });
      toast({ title: "Course updated", tone: "success" });
      setEditItem(null);
      resetForm();
      fetchData();
    } catch (error: any) {
      toast({
        title: "Failed to update course",
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
      await archiveCourse(archiveItem.id);
      toast({ title: "Course archived", tone: "success" });
      setArchiveItem(null);
      fetchData();
    } catch (error: any) {
      toast({
        title: "Failed to archive course",
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
      await publishCourse(id);
      toast({ title: "Course published", tone: "success" });
      fetchData();
    } catch (error: any) {
      toast({
        title: "Failed to publish course",
        description: error.message,
        tone: "error",
      });
    } finally {
      setSaving(false);
    }
  };

  const openEdit = (item: Course) => {
    setEditItem(item);
    setTitle(item.title);
    setDescription(item.description || "");
  };

  return (
    <>
      <PageShell
        title="Courses"
        subtitle="Manage standalone content entities"
        search={{ value: search, onChange: setSearch }}
        onAdd={() => {
          resetForm();
          setCreateOpen(true);
        }}
        addLabel="New Course"
      >
        <div className="flex flex-col gap-2">
          {loading ? (
            Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-3 py-3 bg-white rounded-lg border border-[var(--border)] shadow-sm">
                <Skeleton className="h-8 w-8 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-48" />
                  <Skeleton className="h-3 w-24" />
                </div>
              </div>
            ))
          ) : data.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-8 text-center bg-white rounded-lg border border-dashed border-[var(--border)]">
              <FileText className="h-8 w-8 text-[var(--muted)] mb-3" />
              <p className="text-sm font-medium text-[var(--foreground)]">No courses found</p>
              <p className="text-xs text-[var(--muted)] mt-1">Try adjusting your search or create a new course.</p>
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
                  <FileText className="h-4 w-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium text-[var(--foreground)] truncate">
                      {item.title}
                    </p>
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
        title="Create Course"
      >
        <form onSubmit={(e) => { e.preventDefault(); handleCreate(); }} className="space-y-3">
          <InputField
            label="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
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
        title="Edit Course"
      >
        <form onSubmit={(e) => { e.preventDefault(); handleUpdate(); }} className="space-y-3">
          <InputField
            label="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
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
        title="Archive Course"
        description={`Are you sure you want to archive "${archiveItem?.title}"?`}
        confirmLabel="Archive"
        onConfirm={handleArchive}
        loading={saving}
        destructive
      />
    </>
  );
}
