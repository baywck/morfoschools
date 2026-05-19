"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { 
  listStudents, archiveStudent, restoreStudent, createStudentFull, updateStudent, updateUser, type Student,
  listClassSections, createGuardian, linkStudentGuardian, listGuardians, type ClassSection, type Guardian
} from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { BookOpen, Trash2, Pencil, Plus, X, RotateCcw } from "lucide-react";
import { cn } from "@/lib/cn";

export default function StudentsPage() {
  const { toast } = useToast();
  const [students, setStudents] = useState<Student[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [classFilter, setClassFilter] = useState("");

  // Class options
  const [classes, setClasses] = useState<ClassSection[]>([]);

  // Create sheet
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createForm, setCreateForm] = useState({ displayName: "", email: "", password: "", studentIdNumber: "", gradeLevel: "", classSectionId: "" });
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Edit sheet
  const [editTarget, setEditTarget] = useState<Student | null>(null);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ studentIdNumber: "", gradeLevel: "", status: "", email: "", password: "" });

  const [studentToArchive, setStudentToArchive] = useState<Student | null>(null);
  const [archiving, setArchiving] = useState(false);

  // Guardians
  const [guardians, setGuardians] = useState<Guardian[]>([]);
  const [loadingGuardians, setLoadingGuardians] = useState(false);
  const [showAddGuardian, setShowAddGuardian] = useState(false);
  const [guardianForm, setGuardianForm] = useState({ name: "", phone: "", relationship: "", email: "", password: "" });
  const [addingGuardian, setAddingGuardian] = useState(false);

  async function load() {
    setLoading(true);
    const [res, classesRes] = await Promise.all([
      listStudents({ search: search || undefined, classSectionId: classFilter || undefined }),
      listClassSections()
    ]);
    if (res.data) {
      setStudents(res.data.data);
      setTotal(res.data.pagination.total);
    }
    if (classesRes.data) {
      setClasses(classesRes.data.data);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search, classFilter]);

  useEffect(() => {
    function h() { load(); }
    window.addEventListener("morfoschools:data-changed", h);
    return () => window.removeEventListener("morfoschools:data-changed", h);
  }, []);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    const res = await createStudentFull(createForm);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Student created" });
    setShowCreate(false);
    setCreateForm({ displayName: "", email: "", password: "", studentIdNumber: "", gradeLevel: "", classSectionId: "" });
    setCreating(false);
    load();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  function openEdit(student: Student) {
    setEditTarget(student);
    setEditForm({ 
      studentIdNumber: student.studentIdNumber || "", 
      gradeLevel: student.gradeLevel || "", 
      status: student.status,
      email: student.email || "",
      password: ""
    });
    setFieldErrors({});
    setShowAddGuardian(false);
    loadStudentGuardians(student.id);
  }

  async function loadStudentGuardians(studentId: string) {
    setLoadingGuardians(true);
    // Placeholder logic for listing linked guardians
    // Real implementation would hit an endpoint like /students/:id/guardians
    // Using listGuardians() as placeholder per instructions
    const res = await listGuardians();
    if (res.data) {
      setGuardians(res.data.data);
    }
    setLoadingGuardians(false);
  }

  async function handleAddGuardian() {
    if (!editTarget) return;
    if (!guardianForm.name) {
      toast({ tone: "error", title: "Name is required" });
      return;
    }
    if (!guardianForm.email) {
      toast({ tone: "error", title: "Email is required for guardian login" });
      return;
    }
    if (!guardianForm.password) {
      toast({ tone: "error", title: "Password is required for guardian login" });
      return;
    }
    setAddingGuardian(true);
    // We would ideally have a composite createGuardianFull but let's see if createGuardian works or if we just pass email/password
    // The instructions say: "For guardian create in edit, use createGuardian then linkStudentGuardian."
    // Let's pass all fields to createGuardian
    const gRes = await createGuardian({
      name: guardianForm.name,
      phone: guardianForm.phone,
      relationship: guardianForm.relationship,
      email: guardianForm.email,
      password: guardianForm.password
    } as any);
    
    if (gRes.error) {
      toast({ tone: "error", title: "Failed to create guardian", description: gRes.error.message });
      setAddingGuardian(false);
      return;
    }
    
    const guardianId = gRes.data?.id;
    if (!guardianId) return;
    
    const linkRes = await linkStudentGuardian(guardianId, { studentId: editTarget.id, isPrimary: false });
    if (linkRes.error) {
      toast({ tone: "error", title: "Failed to link guardian", description: linkRes.error.message });
      setAddingGuardian(false);
      return;
    }

    toast({ tone: "success", title: "Guardian added" });
    setGuardianForm({ name: "", phone: "", relationship: "", email: "", password: "" });
    setShowAddGuardian(false);
    setAddingGuardian(false);
    loadStudentGuardians(editTarget.id);
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

    const res = await updateStudent(editTarget.id, { studentIdNumber: editForm.studentIdNumber, gradeLevel: editForm.gradeLevel, status: editForm.status });
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    toast({ tone: "success", title: "Student updated" });
    setEditTarget(null);
    setEditing(false);
    load();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handleArchive(id: string) {
    setArchiving(true);
    const res = await archiveStudent(id);
    setArchiving(false);
    if (res.error) { toast({ tone: "error", title: "Failed", description: res.error.message }); return; }
    toast({ tone: "success", title: "Student archived" });
    setStudentToArchive(null);
    load();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  async function handleRestore(id: string) {
    const res = await restoreStudent(id);
    if (res.error) {
      const emailMsg = res.error.fields?.email;
      toast({ tone: "error", title: "Restore failed", description: emailMsg || res.error.message });
      return;
    }
    toast({ tone: "success", title: "Student restored" });
    load();
    window.dispatchEvent(new Event("morfoschools:data-changed"));
  }

  return (
    <>
    <PageShell
      title="Students"
      subtitle={`${total} student${total !== 1 ? "s" : ""}`}
      search={{ value: search, onChange: setSearch }}
      onAdd={() => setShowCreate(true)}
      addLabel="Add Student"
    >
      <ConfirmDialog
        open={!!studentToArchive}
        onCancel={() => setStudentToArchive(null)}
        onConfirm={() => studentToArchive && handleArchive(studentToArchive.id)}
        title="Archive Student"
        description={`Are you sure you want to archive ${studentToArchive?.displayName}? This action can be undone later.`}
        confirmLabel="Archive Student"
        loading={archiving}
        destructive
      />

      {/* Class filter */}
      {classes.length > 0 && (
        <div className="flex items-center gap-2 mb-4 overflow-x-auto pb-1 scrollbar-none">
          <button
            onClick={() => setClassFilter("")}
            className={`shrink-0 rounded-lg px-3 py-1.5 text-[11px] font-medium transition-colors ${
              classFilter === ""
                ? "bg-[var(--primary)] text-[var(--primary-foreground)]"
                : "bg-[var(--muted)] text-[var(--muted-foreground)] hover:bg-[var(--border)]"
            }`}
          >
            All
          </button>
          {classes.map((c) => (
            <button
              key={c.id}
              onClick={() => setClassFilter(c.id)}
              className={`shrink-0 rounded-lg px-3 py-1.5 text-[11px] font-medium transition-colors ${
                classFilter === c.id
                  ? "bg-[var(--primary)] text-[var(--primary-foreground)]"
                  : "bg-[var(--muted)] text-[var(--muted-foreground)] hover:bg-[var(--border)]"
              }`}
            >
              {c.name}
            </button>
          ))}
        </div>
      )}

      {loading ? (
        <div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}</div>
      ) : students.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
          <BookOpen size={24} className="text-[var(--muted-foreground)] mb-2" />
          <p className="text-[13px] font-semibold text-[var(--foreground)]">No students yet</p>
          <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Register users as students from the Users module.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
          <div className="divide-y divide-[var(--border)]">
            {students.map((s) => (
              <div key={s.id} className="flex items-center gap-4 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--brand-soft)] text-[var(--brand)]">
                  <BookOpen size={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{s.displayName}</p>
                  <p className="text-[11px] text-[var(--muted-foreground)]">{s.gradeLevel || s.email}</p>
                </div>
                {s.studentIdNumber && <span className="text-[10px] text-[var(--muted-foreground)] font-mono">{s.studentIdNumber}</span>}
                <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", s.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]")}>{s.status}</span>
                <RowActions
                  actions={
                    s.status === "archived"
                      ? [
                          { label: "Restore", icon: <RotateCcw size={14} />, onClick: () => handleRestore(s.id) },
                        ]
                      : [
                          { label: "Edit", icon: <Pencil size={14} />, onClick: () => openEdit(s) },
                          { label: "Archive", icon: <Trash2 size={14} />, onClick: () => setStudentToArchive(s), variant: "danger" }
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
    <RightPullSheet open={showCreate} title="Add Student" onClose={() => setShowCreate(false)}>
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
          label="Student ID Number (optional)"
          value={createForm.studentIdNumber}
          onChange={(e) => setCreateForm({ ...createForm, studentIdNumber: e.target.value })}
          error={fieldErrors.studentIdNumber}
        />
        <InputField
          label="Grade Level (optional)"
          value={createForm.gradeLevel}
          onChange={(e) => setCreateForm({ ...createForm, gradeLevel: e.target.value })}
          error={fieldErrors.gradeLevel}
        />
        <SelectField
          label="Class (optional)"
          value={createForm.classSectionId}
          onChange={(val) => setCreateForm({ ...createForm, classSectionId: val })}
          options={[
            { value: "", label: "None" },
            ...classes.map(c => ({ value: c.id, label: c.name }))
          ]}
          error={fieldErrors.classSectionId}
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
    <RightPullSheet open={!!editTarget} title="Edit Student" onClose={() => setEditTarget(null)}>
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
          label="Student ID Number"
          value={editForm.studentIdNumber}
          onChange={(e) => setEditForm({ ...editForm, studentIdNumber: e.target.value })}
          error={fieldErrors.studentIdNumber}
        />
        <InputField
          label="Grade Level"
          value={editForm.gradeLevel}
          onChange={(e) => setEditForm({ ...editForm, gradeLevel: e.target.value })}
          error={fieldErrors.gradeLevel}
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
          <div className="flex items-center justify-between">
            <h3 className="text-[13px] font-semibold text-[var(--foreground)]">Guardians</h3>
            {!showAddGuardian && (
              <button 
                type="button" 
                onClick={() => setShowAddGuardian(true)}
                className="text-[11px] font-medium text-[var(--brand)] hover:underline flex items-center gap-1"
              >
                <Plus size={12} /> Add Guardian
              </button>
            )}
          </div>

          {showAddGuardian && (
            <div className="p-3 rounded-lg border border-[var(--border)] bg-[var(--accent)] space-y-2">
              <InputField
                label="Name *"
                value={guardianForm.name}
                onChange={(e) => setGuardianForm({ ...guardianForm, name: e.target.value })}
              />
              <InputField
                label="Email *"
                value={guardianForm.email}
                onChange={(e) => setGuardianForm({ ...guardianForm, email: e.target.value })}
              />
              <InputField
                label="Password *"
                type="password"
                value={guardianForm.password}
                onChange={(e) => setGuardianForm({ ...guardianForm, password: e.target.value })}
              />
              <InputField
                label="Phone"
                value={guardianForm.phone}
                onChange={(e) => setGuardianForm({ ...guardianForm, phone: e.target.value })}
              />
              <InputField
                label="Relationship"
                value={guardianForm.relationship}
                onChange={(e) => setGuardianForm({ ...guardianForm, relationship: e.target.value })}
              />
              <div className="flex gap-2 justify-end pt-1">
                <button type="button" onClick={() => setShowAddGuardian(false)} className="text-[11px] font-medium text-[var(--muted-foreground)] hover:text-[var(--foreground)] px-2">
                  Cancel
                </button>
                <button 
                  type="button" 
                  onClick={handleAddGuardian}
                  disabled={addingGuardian}
                  className="bg-[var(--brand)] text-white text-[11px] px-2.5 py-1 rounded-md font-medium hover:opacity-90 disabled:opacity-50 flex items-center gap-1"
                >
                  {addingGuardian && <span className="h-3 w-3 animate-spin rounded-full border-2 border-current border-r-transparent" />}
                  Save
                </button>
              </div>
            </div>
          )}

          <div className="space-y-2">
            {loadingGuardians ? (
              <Skeleton className="h-10 w-full" />
            ) : guardians.length === 0 ? (
              <p className="text-[12px] text-[var(--muted-foreground)]">No guardians linked.</p>
            ) : (
              guardians.map(g => (
                <div key={g.id} className="flex items-center justify-between p-2.5 rounded-lg border border-[var(--border)] bg-[var(--card)]">
                  <div>
                    <p className="text-[12px] font-medium text-[var(--foreground)]">{g.name}</p>
                    <p className="text-[11px] text-[var(--muted-foreground)]">
                      {g.relationship || "Unknown"} • {g.phone || "No phone"}
                    </p>
                  </div>
                  <button 
                    type="button" 
                    title="Remove guardian link (UI placeholder)"
                    onClick={() => toast({ tone: "info", title: "Not implemented", description: "Unlinking guardians requires backend support" })}
                    className="p-1.5 text-[var(--muted-foreground)] hover:text-[var(--destructive)] hover:bg-[var(--destructive-soft)] rounded-md transition-colors"
                  >
                    <X size={14} />
                  </button>
                </div>
              ))
            )}
          </div>
        </div>

        <div className="flex gap-2 justify-end pt-3 border-t border-[var(--border)]">
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
