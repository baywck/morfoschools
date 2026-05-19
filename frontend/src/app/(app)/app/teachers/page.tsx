"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { 
  listTeachers, archiveTeacher, restoreTeacher, createTeacherFull, updateTeacher, updateUser, type Teacher,
  listTeacherSubjects, assignTeacherSubject, unassignTeacherSubject, listSubjects, type TeacherSubject, type Subject
} from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { GraduationCap, Trash2, Pencil, Plus, X, RotateCcw } from "lucide-react";
import { cn } from "@/lib/cn";

export default function TeachersPage() {
  const { toast } = useToast();
  const [teachers, setTeachers] = useState<Teacher[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  
  // Create sheet
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createForm, setCreateForm] = useState({ displayName: "", email: "", password: "", employeeId: "", specialization: "", subjectIds: [] as string[] });
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Edit sheet
  const [editTarget, setEditTarget] = useState<Teacher | null>(null);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ employeeId: "", specialization: "", status: "", email: "", password: "" });

  const [teacherToArchive, setTeacherToArchive] = useState<Teacher | null>(null);
  const [archiving, setArchiving] = useState(false);

  // Subjects
  const [assignedSubjects, setAssignedSubjects] = useState<TeacherSubject[]>([]);
  const [availableSubjects, setAvailableSubjects] = useState<Subject[]>([]);
  const [loadingSubjects, setLoadingSubjects] = useState(false);
  const [subjectToAdd, setSubjectToAdd] = useState("");
  const [assigningSubject, setAssigningSubject] = useState(false);
  const [unassigningSubject, setUnassigningSubject] = useState<string | null>(null);

  // Map of teacher subjects for list display
  const [teacherSubjectsMap, setTeacherSubjectsMap] = useState<Record<string, TeacherSubject[]>>({});

  async function load() {
    setLoading(true);
    const res = await listTeachers({ search: search || undefined });
    if (res.data) {
      setTeachers(res.data.data);
      setTotal(res.data.pagination.total);
      // Load subjects for all teachers
      const map: Record<string, TeacherSubject[]> = {};
      await Promise.all(res.data.data.map(async (t) => {
        const tsRes = await listTeacherSubjects(t.id);
        if (tsRes.data) map[t.id] = tsRes.data.data;
      }));
      setTeacherSubjectsMap(map);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  useEffect(() => {
    function h() { load(); }
    window.addEventListener("morfoschools:data-changed", h);
    return () => window.removeEventListener("morfoschools:data-changed", h);
  }, []);

  useEffect(() => {
    async function loadAllSubjects() {
      const res = await listSubjects();
      if (res.data) setAvailableSubjects(res.data.data);
    }
    loadAllSubjects();
  }, []);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    const res = await createTeacherFull(createForm);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Teacher created" });
    setShowCreate(false);
    setCreateForm({ displayName: "", email: "", password: "", employeeId: "", specialization: "", subjectIds: [] });
    setCreating(false);
    load();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  function toggleSubject(id: string) {
    setCreateForm(prev => {
      if (prev.subjectIds.includes(id)) {
        return { ...prev, subjectIds: prev.subjectIds.filter(s => s !== id) };
      }
      return { ...prev, subjectIds: [...prev.subjectIds, id] };
    });
  }

  function openEdit(teacher: Teacher) {
    setEditTarget(teacher);
    setEditForm({ 
      employeeId: teacher.employeeId || "", 
      specialization: teacher.specialization || "", 
      status: teacher.status,
      email: teacher.email || "",
      password: ""
    });
    setFieldErrors({});
    loadTeacherSubjects(teacher.id);
  }

  async function loadTeacherSubjects(teacherId: string) {
    setLoadingSubjects(true);
    const tsRes = await listTeacherSubjects(teacherId);
    if (tsRes.data) setAssignedSubjects(tsRes.data.data);
    setLoadingSubjects(false);
  }

  async function handleAssignSubject(subjectId: string) {
    if (!editTarget || !subjectId) return;
    setSubjectToAdd(""); // reset immediately
    setAssigningSubject(true);
    const res = await assignTeacherSubject(editTarget.id, subjectId);
    setAssigningSubject(false);
    if (res.error) {
      toast({ tone: "error", title: "Failed to assign subject", description: res.error.message });
      return;
    }
    loadTeacherSubjects(editTarget.id);
  }

  async function handleUnassignSubject(subjectId: string) {
    if (!editTarget) return;
    setUnassigningSubject(subjectId);
    const res = await unassignTeacherSubject(editTarget.id, subjectId);
    setUnassigningSubject(null);
    if (res.error) {
      toast({ tone: "error", title: "Failed to unassign subject", description: res.error.message });
      return;
    }
    loadTeacherSubjects(editTarget.id);
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setFieldErrors({});
    setEditing(true);

    // Update user-level fields (email, password)
    const userUpdate: Record<string, string> = {};
    if (editForm.email && editForm.email !== editTarget.email) userUpdate.email = editForm.email;
    if (editForm.password) userUpdate.password = editForm.password;
    if (Object.keys(userUpdate).length > 0) {
      const userRes = await updateUser(editTarget.userId, userUpdate);
      if (userRes.error) {
        if (userRes.error.fields) setFieldErrors(userRes.error.fields);
        else toast({ tone: "error", title: "Failed", description: userRes.error.message });
        setEditing(false);
        return;
      }
    }

    // Update teacher-level fields
    const res = await updateTeacher(editTarget.id, { 
      employeeId: editForm.employeeId, 
      specialization: editForm.specialization, 
      status: editForm.status 
    });
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    toast({ tone: "success", title: "Teacher updated" });
    setEditTarget(null);
    setEditing(false);
    load();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handleArchive(id: string) {
    setArchiving(true);
    const res = await archiveTeacher(id);
    setArchiving(false);
    if (res.error) { toast({ tone: "error", title: "Failed", description: res.error.message }); return; }
    toast({ tone: "success", title: "Teacher archived" });
    setTeacherToArchive(null);
    load();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handleRestore(id: string) {
    const res = await restoreTeacher(id);
    if (res.error) {
      const emailMsg = res.error.fields?.email;
      toast({ tone: "error", title: "Restore failed", description: emailMsg || res.error.message });
      return;
    }
    toast({ tone: "success", title: "Teacher restored" });
    load();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  return (
    <>
    <PageShell
      title="Teachers"
      subtitle={`${total} teacher${total !== 1 ? "s" : ""}`}
      search={{ value: search, onChange: setSearch }}
      onAdd={() => setShowCreate(true)}
      addLabel="Add Teacher"
    >
      <ConfirmDialog
        open={!!teacherToArchive}
        onCancel={() => setTeacherToArchive(null)}
        onConfirm={() => teacherToArchive && handleArchive(teacherToArchive.id)}
        title="Archive Teacher"
        description={`Are you sure you want to archive ${teacherToArchive?.displayName}? This action can be undone later.`}
        confirmLabel="Archive Teacher"
        loading={archiving}
        destructive
      />

      {loading ? (
        <div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}</div>
      ) : teachers.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
          <GraduationCap size={24} className="text-[var(--muted-foreground)] mb-2" />
          <p className="text-[13px] font-semibold text-[var(--foreground)]">No teachers yet</p>
          <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Register users as teachers from the Users module.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
          <div className="divide-y divide-[var(--border)]">
            {teachers.map((t) => (
              <div key={t.id} className="flex items-center gap-4 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--info-soft)] text-[var(--info)]">
                  <GraduationCap size={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{t.displayName}</p>
                  <p className="text-[11px] text-[var(--muted-foreground)] truncate">
                    {teacherSubjectsMap[t.id]?.length
                      ? teacherSubjectsMap[t.id].map(s => s.name).join(", ")
                      : t.email}
                  </p>
                </div>
                {t.employeeId && <span className="text-[10px] text-[var(--muted-foreground)] font-mono">{t.employeeId}</span>}
                <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", t.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]")}>{t.status}</span>
                <RowActions
                  actions={
                    t.status === "archived"
                      ? [
                          { label: "Restore", icon: <RotateCcw size={14} />, onClick: () => handleRestore(t.id) },
                        ]
                      : [
                          { label: "Edit", icon: <Pencil size={14} />, onClick: () => openEdit(t) },
                          { label: "Archive", icon: <Trash2 size={14} />, onClick: () => setTeacherToArchive(t), variant: "danger" }
                        ]
                  }
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </PageShell>

    {/* Create Sheet */}
    <RightPullSheet open={showCreate} title="Add Teacher" onClose={() => setShowCreate(false)}>
      <form onSubmit={handleCreate} className="space-y-3">
        <InputField
          label="Display Name"
          value={createForm.displayName}
          onChange={(e) => setCreateForm({ ...createForm, displayName: e.target.value })}
          error={fieldErrors.displayName}
        />
        <InputField
          label="Email"
          value={createForm.email}
          onChange={(e) => setCreateForm({ ...createForm, email: e.target.value })}
          error={fieldErrors.email}
        />
        <InputField
          label="Password"
          type="password"
          value={createForm.password}
          onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })}
          error={fieldErrors.password}
        />
        <InputField
          label="Employee ID (optional)"
          value={createForm.employeeId}
          onChange={(e) => setCreateForm({ ...createForm, employeeId: e.target.value })}
          error={fieldErrors.employeeId}
        />
        <InputField
          label="Specialization (optional)"
          value={createForm.specialization}
          onChange={(e) => setCreateForm({ ...createForm, specialization: e.target.value })}
          error={fieldErrors.specialization}
        />
        
        <div className="pt-2">
          <p className="text-[12px] font-medium text-[var(--foreground)] mb-2">Subjects</p>
          <div className="flex flex-wrap gap-2">
            {availableSubjects.length === 0 ? (
              <p className="text-[12px] text-[var(--muted-foreground)]">No subjects available.</p>
            ) : (
              availableSubjects.map((s) => {
                const selected = createForm.subjectIds.includes(s.id);
                return (
                  <button
                    key={s.id}
                    type="button"
                    onClick={() => toggleSubject(s.id)}
                    className={cn(
                      "inline-flex items-center rounded-md px-2.5 py-1 text-[11px] font-medium transition-colors border",
                      selected
                        ? "bg-[var(--brand-soft)] text-[var(--brand)] border-transparent"
                        : "bg-[var(--muted)] text-[var(--muted-foreground)] border-transparent hover:border-[var(--border-strong)]"
                    )}
                  >
                    {s.name}
                  </button>
                );
              })
            )}
          </div>
        </div>

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
    <RightPullSheet open={!!editTarget} title="Edit Teacher" onClose={() => setEditTarget(null)}>
      <form onSubmit={handleEdit} className="space-y-3">
        <InputField
          label="Email"
          type="email"
          value={editForm.email}
          onChange={(e) => setEditForm({ ...editForm, email: e.target.value })}
          error={fieldErrors.email}
        />
        <InputField
          label="New Password (leave blank to keep)"
          type="password"
          value={editForm.password}
          onChange={(e) => setEditForm({ ...editForm, password: e.target.value })}
          error={fieldErrors.password}
        />
        <InputField
          label="Employee ID"
          value={editForm.employeeId}
          onChange={(e) => setEditForm({ ...editForm, employeeId: e.target.value })}
          error={fieldErrors.employeeId}
        />
        <InputField
          label="Specialization"
          value={editForm.specialization}
          onChange={(e) => setEditForm({ ...editForm, specialization: e.target.value })}
          error={fieldErrors.specialization}
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
        
        <div className="pt-4 border-t border-[var(--border)] space-y-3 mt-4">
          <h3 className="text-[13px] font-semibold text-[var(--foreground)]">Assigned Subjects</h3>
          
          <div className="flex flex-wrap gap-2">
            {loadingSubjects ? (
              <Skeleton className="h-6 w-20" />
            ) : assignedSubjects.length === 0 ? (
              <p className="text-[12px] text-[var(--muted-foreground)]">No subjects assigned.</p>
            ) : (
              assignedSubjects.map(ts => {
                const subjectName = ts.name || availableSubjects.find(s => s.id === ts.id)?.name || 'Unknown Subject';
                const isRemoving = unassigningSubject === ts.id;
                return (
                  <div key={ts.id} className={cn("inline-flex items-center gap-1 rounded-md pl-2 pr-1 py-0.5 text-[11px] font-medium bg-[var(--brand-soft)] text-[var(--brand)]", isRemoving && "opacity-50 pointer-events-none")}>
                    {subjectName}
                    {isRemoving ? (
                       <span className="h-3 w-3 animate-spin rounded-full border border-current border-r-transparent ml-1" />
                    ) : (
                      <button 
                        type="button" 
                        onClick={() => handleUnassignSubject(ts.id)}
                        className="p-0.5 hover:bg-[var(--brand)] hover:text-white rounded transition-colors ml-1"
                      >
                        <X size={12} />
                      </button>
                    )}
                  </div>
                );
              })
            )}
          </div>

          {!loadingSubjects && (
            <div className="relative">
              <SelectField
                label="Add Subject"
                value={subjectToAdd}
                disabled={assigningSubject}
                onChange={(val) => {
                  setSubjectToAdd(val);
                  if (val) handleAssignSubject(val);
                }}
                options={[
                  { value: "", label: "Select a subject..." },
                  ...availableSubjects
                    .filter(s => !assignedSubjects.some(ts => ts.id === s.id))
                    .map(s => ({ value: s.id, label: s.name }))
                ]}
              />
              {assigningSubject && (
                <div className="absolute top-8 right-2">
                  <span className="block h-4 w-4 animate-spin rounded-full border-2 border-[var(--primary)] border-r-transparent" />
                </div>
              )}
            </div>
          )}
        </div>

        <div className="flex gap-2 justify-end pt-3">
          <button type="button" onClick={() => setEditTarget(null)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">
            Cancel
          </button>
          <button type="submit" disabled={editing} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
            {editing && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
            Save Changes
          </button>
        </div>
      </form>
    </RightPullSheet>
    </>
  );
}
