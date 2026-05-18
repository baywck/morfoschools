"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { 
  listAcademicYears, 
  createAcademicYear, 
  updateAcademicYear, 
  archiveAcademicYear, 
  type AcademicYear
} from "@/lib/modules-api";
import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RowActions } from "@/components/ui/row-actions";
import { PageShell } from "@/components/layout/page-shell";
import { Plus, CalendarRange, Trash2, Pencil } from "lucide-react";
import { cn } from "@/lib/cn";

export default function AcademicYearsPage() {
  const { toast } = useToast();
  const [academicYears, setAcademicYears] = useState<AcademicYear[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  // Create sheet
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newCode, setNewCode] = useState("");
  const [newName, setNewName] = useState("");
  const [newStartsOn, setNewStartsOn] = useState("");
  const [newEndsOn, setNewEndsOn] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  // Edit sheet
  const [editTarget, setEditTarget] = useState<AcademicYear | null>(null);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ 
    name: "", 
    startsOn: "", 
    endsOn: "", 
    status: "" 
  });

  // Confirm dialogs
  const [archiveTarget, setArchiveTarget] = useState<AcademicYear | null>(null);
  const [archiving, setArchiving] = useState(false);

  async function loadData() {
    setLoading(true);
    const res = await listAcademicYears();
    
    if (res.data) {
      setAcademicYears(res.data.data);
      setTotal(res.data.pagination?.total || res.data.data.length);
    }
    
    setLoading(false);
  }

  useEffect(() => { loadData(); }, []);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    setCreating(true);
    
    const res = await createAcademicYear({ 
      code: newCode, 
      name: newName, 
      startsOn: newStartsOn || undefined,
      endsOn: newEndsOn || undefined
    });
    
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setCreating(false);
      return;
    }
    toast({ tone: "success", title: "Academic year created" });
    setShowCreate(false);
    setNewCode("");
    setNewName("");
    setNewStartsOn("");
    setNewEndsOn("");
    setCreating(false);
    loadData();
  }

  function openEdit(ay: AcademicYear) {
    setEditTarget(ay);
    setEditForm({ 
      name: ay.name, 
      startsOn: ay.startsOn ? ay.startsOn.split('T')[0] : "", 
      endsOn: ay.endsOn ? ay.endsOn.split('T')[0] : "", 
      status: ay.status 
    });
    setFieldErrors({});
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setFieldErrors({});
    setEditing(true);
    
    const res = await updateAcademicYear(editTarget.id, {
      name: editForm.name,
      startsOn: editForm.startsOn || undefined,
      endsOn: editForm.endsOn || undefined,
      status: editForm.status
    });
    
    if (res.error) {
      if (res.error.fields) setFieldErrors(res.error.fields);
      else toast({ tone: "error", title: "Failed", description: res.error.message });
      setEditing(false);
      return;
    }
    
    toast({ tone: "success", title: "Academic year updated" });
    setEditTarget(null);
    setEditing(false);
    loadData();
  }

  async function confirmArchive() {
    if (!archiveTarget) return;
    setArchiving(true);
    const res = await archiveAcademicYear(archiveTarget.id);
    if (res.error) {
      toast({ tone: "error", title: "Failed", description: res.error.message });
    } else {
      toast({ tone: "success", title: "Academic year archived" });
      loadData();
    }
    setArchiving(false);
    setArchiveTarget(null);
  }

  return (
    <>
      <PageShell
        title="Academic Years"
        subtitle={`${total} academic year${total !== 1 ? "s" : ""} registered`}
        onAdd={() => setShowCreate(true)}
        addLabel="Add Year"
      >
        {/* List */}
        {loading ? (
          <div className="space-y-2">
            {[1, 2].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : academicYears.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <CalendarRange size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No academic years yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Create your first academic year to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {academicYears.map((ay) => (
                <div key={ay.id} className="flex items-center gap-3 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">
                    <CalendarRange size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{ay.name}</p>
                      <span className="rounded-md bg-[var(--accent)] px-1.5 py-0.5 text-[10px] text-[var(--muted-foreground)]">
                        {ay.code}
                      </span>
                    </div>
                    {(ay.startsOn || ay.endsOn) && (
                      <p className="text-[11px] text-[var(--muted-foreground)] mt-0.5">
                        {ay.startsOn ? new Date(ay.startsOn).toLocaleDateString() : '?'} - {ay.endsOn ? new Date(ay.endsOn).toLocaleDateString() : '?'}
                      </p>
                    )}
                  </div>
                  <span className={cn(
                    "rounded-md px-2 py-0.5 text-[10px] font-medium",
                    ay.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]"
                  )}>
                    {ay.status}
                  </span>
                  <RowActions actions={[
                    { label: "Edit", icon: <Pencil size={13} />, onClick: () => openEdit(ay) },
                    { label: "Archive", icon: <Trash2 size={13} />, onClick: () => setArchiveTarget(ay), variant: "danger" },
                  ]} />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create Sheet */}
      <RightPullSheet open={showCreate} title="Add Year" onClose={() => setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField
            label="Code"
            value={newCode}
            onChange={(e) => setNewCode(e.target.value)}
            error={fieldErrors.code}
            helperText="e.g. 2025-2026"
            prefix={<CalendarRange size={14} />}
          />
          <InputField
            label="Name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            error={fieldErrors.name}
            helperText="e.g. Tahun Ajaran 2025/2026"
          />
          <InputField
            label="Starts On"
            type="date"
            value={newStartsOn}
            onChange={(e) => setNewStartsOn(e.target.value)}
            error={fieldErrors.startsOn}
          />
          <InputField
            label="Ends On"
            type="date"
            value={newEndsOn}
            onChange={(e) => setNewEndsOn(e.target.value)}
            error={fieldErrors.endsOn}
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
      <RightPullSheet open={!!editTarget} title="Edit Year" onClose={() => setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField
            label="Name"
            value={editForm.name}
            onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
            error={fieldErrors.name}
            prefix={<CalendarRange size={14} />}
          />
          <InputField
            label="Starts On"
            type="date"
            value={editForm.startsOn}
            onChange={(e) => setEditForm({ ...editForm, startsOn: e.target.value })}
            error={fieldErrors.startsOn}
          />
          <InputField
            label="Ends On"
            type="date"
            value={editForm.endsOn}
            onChange={(e) => setEditForm({ ...editForm, endsOn: e.target.value })}
            error={fieldErrors.endsOn}
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
        title="Archive Academic Year"
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
