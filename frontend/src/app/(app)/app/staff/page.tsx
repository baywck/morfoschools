"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { listStaff, archiveStaff, createStaffFull, updateStaff, type Staff } from "@/lib/modules-api";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { Briefcase, Trash2, Pencil, Plus } from "lucide-react";
import { cn } from "@/lib/cn";

export default function StaffPage() {
  const { toast } = useToast();
  const [staff, setStaff] = useState<Staff[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  // Create sheet
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createForm, setCreateForm] = useState({ displayName: "", email: "", password: "", employeeId: "", department: "", position: "" });
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Edit sheet
  const [editTarget, setEditTarget] = useState<Staff | null>(null);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ employeeId: "", department: "", position: "", status: "" });

  const [staffToArchive, setStaffToArchive] = useState<Staff | null>(null);

  async function load() {
    setLoading(true);
    const res = await listStaff({ search: search || undefined });
    if (res.data) {
      setStaff(res.data.data);
      setTotal(res.data.pagination.total);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    const res = await createStaffFull(createForm);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Staff created" });
    setShowCreate(false);
    setCreateForm({ displayName: "", email: "", password: "", employeeId: "", department: "", position: "" });
    setCreating(false);
    load();
  }

  function openEdit(staff: Staff) {
    setEditTarget(staff);
    setEditForm({ 
      employeeId: staff.employeeId || "", 
      department: staff.department || "", 
      position: staff.position || "", 
      status: staff.status 
    });
    setFieldErrors({});
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setFieldErrors({});
    setEditing(true);
    const res = await updateStaff(editTarget.id, editForm);
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    toast({ tone: "success", title: "Staff updated" });
    setEditTarget(null);
    setEditing(false);
    load();
  }

  async function handleArchive(id: string) {
    const res = await archiveStaff(id);
    if (res.error) { toast({ tone: "error", title: "Failed", description: res.error.message }); return; }
    toast({ tone: "success", title: "Staff archived" });
    setStaffToArchive(null);
    load();
  }

  return (
    <>
    <PageShell
      title="Staff"
      subtitle={`${total} staff member${total !== 1 ? "s" : ""}`}
      search={{ value: search, onChange: setSearch }}
      onAdd={() => setShowCreate(true)}
      addLabel="Add Staff"
    >
      <ConfirmDialog
        open={!!staffToArchive}
        onCancel={() => setStaffToArchive(null)}
        onConfirm={() => staffToArchive && handleArchive(staffToArchive.id)}
        title="Archive Staff"
        description={`Are you sure you want to archive ${staffToArchive?.displayName}? This action can be undone later.`}
        confirmLabel="Archive Staff"
        destructive
      />

      {loading ? (
        <div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-16 w-full" />)}</div>
      ) : staff.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
          <Briefcase size={24} className="text-[var(--muted-foreground)] mb-2" />
          <p className="text-[13px] font-semibold text-[var(--foreground)]">No staff yet</p>
          <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Register users as staff from the Users module.</p>
        </div>
      ) : (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
          <div className="divide-y divide-[var(--border)]">
            {staff.map((s) => (
              <div key={s.id} className="flex items-center gap-4 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--warning-soft)] text-[var(--warning)]">
                  <Briefcase size={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{s.displayName}</p>
                  <p className="text-[11px] text-[var(--muted-foreground)]">{[s.position, s.department].filter(Boolean).join(" • ") || s.email}</p>
                </div>
                <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", s.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]")}>{s.status}</span>
                <RowActions
                  actions={[
                    { label: "Edit", icon: <Pencil size={14} />, onClick: () => openEdit(s) },
                    { label: "Archive", icon: <Trash2 size={14} />, onClick: () => setStaffToArchive(s), variant: "danger" }
                  ]}
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </PageShell>

    {/* Create Sheet */}
    <RightPullSheet open={showCreate} title="Add Staff" onClose={() => setShowCreate(false)}>
      <form onSubmit={handleCreate} className="space-y-3">
        <InputField
          label="Display Name"
          value={createForm.displayName}
          onChange={(e) => setCreateForm({ ...createForm, displayName: e.target.value })}
          error={fieldErrors.displayName}
          required
        />
        <InputField
          label="Email"
          type="email"
          value={createForm.email}
          onChange={(e) => setCreateForm({ ...createForm, email: e.target.value })}
          error={fieldErrors.email}
          required
        />
        <InputField
          label="Password"
          type="password"
          value={createForm.password}
          onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })}
          error={fieldErrors.password}
          required
        />
        <InputField
          label="Employee ID (optional)"
          value={createForm.employeeId}
          onChange={(e) => setCreateForm({ ...createForm, employeeId: e.target.value })}
          error={fieldErrors.employeeId}
        />
        <InputField
          label="Department (optional)"
          value={createForm.department}
          onChange={(e) => setCreateForm({ ...createForm, department: e.target.value })}
          error={fieldErrors.department}
        />
        <InputField
          label="Position (optional)"
          value={createForm.position}
          onChange={(e) => setCreateForm({ ...createForm, position: e.target.value })}
          error={fieldErrors.position}
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
    <RightPullSheet open={!!editTarget} title="Edit Staff" onClose={() => setEditTarget(null)}>
      <form onSubmit={handleEdit} className="space-y-3">
        <InputField
          label="Employee ID"
          value={editForm.employeeId}
          onChange={(e) => setEditForm({ ...editForm, employeeId: e.target.value })}
          error={fieldErrors.employeeId}
        />
        <InputField
          label="Department"
          value={editForm.department}
          onChange={(e) => setEditForm({ ...editForm, department: e.target.value })}
          error={fieldErrors.department}
        />
        <InputField
          label="Position"
          value={editForm.position}
          onChange={(e) => setEditForm({ ...editForm, position: e.target.value })}
          error={fieldErrors.position}
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
    </>
  );
}
