"use client";

import { useState, useEffect } from "react";
import { useAuth } from "@/lib/auth-provider";
import { useToast } from "@/components/ui/toast";
import { 
  listClassSections, 
  createClassSection, 
  updateClassSection, 
  archiveClassSection, 
  listTeachers,
  type ClassSection,
  type Teacher
} from "@/lib/modules-api";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RowActions } from "@/components/ui/row-actions";
import { PageShell } from "@/components/layout/page-shell";
import { Plus, School2, Trash2, Pencil } from "lucide-react";
import { cn } from "@/lib/cn";

export default function ClassesPage() {
  const { toast } = useToast();
  const [classes, setClasses] = useState<ClassSection[]>([]);
  const [teachers, setTeachers] = useState<Teacher[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  // Create sheet
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newGradeLevel, setNewGradeLevel] = useState("");
  const [newAcademicYearId, setNewAcademicYearId] = useState("");
  const [newHomeroomTeacherId, setNewHomeroomTeacherId] = useState("");
  const [newCapacity, setNewCapacity] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Edit sheet
  const [editTarget, setEditTarget] = useState<ClassSection | null>(null);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ 
    name: "", 
    gradeLevel: "", 
    homeroomTeacherId: "", 
    capacity: "", 
    status: "" 
  });

  // Confirm dialogs
  const [archiveTarget, setArchiveTarget] = useState<ClassSection | null>(null);
  const [archiving, setArchiving] = useState(false);

  async function loadData() {
    setLoading(true);
    const [classesRes, teachersRes] = await Promise.all([
      listClassSections({ search: search || undefined }),
      listTeachers() // Need active teachers for homeroom selection
    ]);
    
    if (classesRes.data) {
      setClasses(classesRes.data.data);
      setTotal(classesRes.data.pagination.total);
    }
    
    if (teachersRes.data) {
      setTeachers(teachersRes.data.data);
    }
    
    setLoading(false);
  }

  useEffect(() => { loadData(); }, [search]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    
    const capacityVal = newCapacity ? parseInt(newCapacity, 10) : undefined;
    
    const res = await createClassSection({ 
      name: newName, 
      gradeLevel: newGradeLevel, 
      academicYearId: newAcademicYearId,
      homeroomTeacherId: newHomeroomTeacherId || undefined,
      capacity: capacityVal
    });
    
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Class created" });
    setShowCreate(false);
    setNewName("");
    setNewGradeLevel("");
    setNewAcademicYearId("");
    setNewHomeroomTeacherId("");
    setNewCapacity("");
    setCreating(false);
    loadData();
  }

  function openEdit(cls: ClassSection) {
    setEditTarget(cls);
    setEditForm({ 
      name: cls.name, 
      gradeLevel: cls.gradeLevel,
      homeroomTeacherId: cls.homeroomTeacherId || "",
      capacity: cls.capacity ? cls.capacity.toString() : "",
      status: cls.status 
    });
    setFieldErrors({});
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setFieldErrors({});
    setEditing(true);
    
    const capacityVal = editForm.capacity ? parseInt(editForm.capacity, 10) : undefined;
    
    const res = await updateClassSection(editTarget.id, {
      name: editForm.name,
      gradeLevel: editForm.gradeLevel,
      homeroomTeacherId: editForm.homeroomTeacherId || undefined,
      capacity: capacityVal,
      status: editForm.status
    });
    
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    
    toast({ tone: "success", title: "Class updated" });
    setEditTarget(null);
    setEditing(false);
    loadData();
  }

  async function confirmArchive() {
    if (!archiveTarget) return;
    setArchiving(true);
    const res = await archiveClassSection(archiveTarget.id);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
    } else {
      toast({ tone: "success", title: "Class archived" });
      loadData();
    }
    setArchiving(false);
    setArchiveTarget(null);
  }

  const teacherOptions = [
    { value: "", label: "None" },
    ...teachers.map(t => ({ value: t.id, label: t.displayName }))
  ];

  function getTeacherName(id: string | null) {
    if (!id) return "No homeroom teacher";
    const teacher = teachers.find(t => t.id === id);
    return teacher ? teacher.displayName : "Unknown teacher";
  }

  return (
    <>
      <PageShell
        title="Class Sections"
        subtitle={`${total} class${total !== 1 ? "es" : ""} registered`}
        search={{ value: search, onChange: setSearch, placeholder: "Search classes..." }}
        onAdd={() => setShowCreate(true)}
        addLabel="Add Class"
      >
        {/* List */}
        {loading ? (
          <div className="space-y-2">
            {[1, 2, 3].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : classes.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <School2 size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No classes yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Create your first class to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {classes.map((c) => (
                <div key={c.id} className="flex items-center gap-3 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">
                    <School2 size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{c.name}</p>
                      <span className="rounded-md bg-[var(--accent)] px-1.5 py-0.5 text-[10px] text-[var(--muted-foreground)]">
                        Grade {c.gradeLevel}
                      </span>
                    </div>
                    <p className="text-[11px] text-[var(--muted-foreground)]">
                      {getTeacherName(c.homeroomTeacherId)}
                    </p>
                  </div>
                  <span className={cn(
                    "rounded-md px-2 py-0.5 text-[10px] font-medium",
                    c.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  )}>
                    {c.status}
                  </span>
                  <RowActions actions={[
                    { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEdit(c) },
                    { label: "Archive", icon: <Trash2 size={13} />, onClick: () => setArchiveTarget(c), variant: "danger" },
                  ]} />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create Sheet */}
      <RightPullSheet open={showCreate} title="Add Class" onClose={() => setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField
            label="Class Name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            error={fieldErrors.name}
            prefix={<School2 size={14} />}
            helperText="e.g. 10-A, 11 Science 1"
          />
          <InputField
            label="Grade Level"
            value={newGradeLevel}
            onChange={(e) => setNewGradeLevel(e.target.value)}
            error={fieldErrors.gradeLevel}
          />
          <InputField
            label="Academic Year ID"
            value={newAcademicYearId}
            onChange={(e) => setNewAcademicYearId(e.target.value)}
            error={fieldErrors.academicYearId}
          />
          <SelectField
            label="Homeroom Teacher"
            value={newHomeroomTeacherId}
            onChange={(val) => setNewHomeroomTeacherId(val)}
            options={teacherOptions}
            error={fieldErrors.homeroomTeacherId}
          />
          <InputField
            label="Capacity"
            type="number"
            value={newCapacity}
            onChange={(e) => setNewCapacity(e.target.value)}
            error={fieldErrors.capacity}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => setShowCreate(false)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={creating} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {creating && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              <Plus size={14} /> Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit Sheet */}
      <RightPullSheet open={!!editTarget} title="Edit Class" onClose={() => setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField
            label="Class Name"
            value={editForm.name}
            onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
            error={fieldErrors.name}
            prefix={<School2 size={14} />}
          />
          <InputField
            label="Grade Level"
            value={editForm.gradeLevel}
            onChange={(e) => setEditForm({ ...editForm, gradeLevel: e.target.value })}
            error={fieldErrors.gradeLevel}
          />
          <SelectField
            label="Homeroom Teacher"
            value={editForm.homeroomTeacherId}
            onChange={(val) => setEditForm({ ...editForm, homeroomTeacherId: val })}
            options={teacherOptions}
            error={fieldErrors.homeroomTeacherId}
          />
          <InputField
            label="Capacity"
            type="number"
            value={editForm.capacity}
            onChange={(e) => setEditForm({ ...editForm, capacity: e.target.value })}
            error={fieldErrors.capacity}
          />
          <SelectField
            label="Status"
            value={editForm.status}
            onChange={(val) => setEditForm({ ...editForm, status: val })}
            options={[
              { value: "active", label: "Active" },
              { value: "inactive", label: "Inactive" },
              { value: "archived", label: "Archived" }
            ]}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => setEditTarget(null)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
              Cancel
            </button>
            <button type="submit" disabled={editing} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {editing ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : "Save Changes"}
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Archive Confirm */}
      <ConfirmDialog
        open={!!archiveTarget}
        title="Archive Class"
        description={`Are you sure you want to archive "${archiveTarget?.name}"?`}
        confirmLabel="Archive"
        destructive
        loading={archiving}
        onConfirm={confirmArchive}
        onCancel={() => setArchiveTarget(null)}
      />
    </>
  );
}
