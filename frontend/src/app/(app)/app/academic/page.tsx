"use client";

import { useState } from "react";
import { useCRUD } from "@/lib/use-crud";
import {
  listAcademicYears,
  createAcademicYear,
  updateAcademicYear,
  archiveAcademicYear,
  type AcademicYear,
} from "@/lib/modules-api";
import { InputField } from "@/components/ui/input-field";
import { DatePicker } from "@/components/ui/date-picker";
import { SelectField } from "@/components/ui/select-field";
import { Skeleton } from "@/components/ui/skeleton";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { RowActions } from "@/components/ui/row-actions";
import { PageShell } from "@/components/layout/page-shell";
import { Plus, CalendarRange, Trash2, Pencil } from "lucide-react";
import { cn } from "@/lib/cn";

export default function AcademicYearsPage() {
  const crud = useCRUD<AcademicYear>({
    name: "Academic Year",
    list: listAcademicYears,
    create: createAcademicYear,
    update: updateAcademicYear,
    archive: archiveAcademicYear,
  });

  const [createForm, setCreateForm] = useState({ code: "", name: "", startsOn: "", endsOn: "" });
  const [editForm, setEditForm] = useState({ name: "", startsOn: "", endsOn: "", status: "" });

  function openEdit(ay: AcademicYear) {
    crud.setEditTarget(ay);
    crud.setFieldErrors({});
    setEditForm({
      name: ay.name,
      startsOn: ay.startsOn ? ay.startsOn.split("T")[0] : "",
      endsOn: ay.endsOn ? ay.endsOn.split("T")[0] : "",
      status: ay.status,
    });
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const success = await crud.handleCreate({
      code: createForm.code,
      name: createForm.name,
      startsOn: createForm.startsOn || undefined,
      endsOn: createForm.endsOn || undefined,
    });
    if (success) setCreateForm({ code: "", name: "", startsOn: "", endsOn: "" });
  }

  async function handleEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!crud.editTarget) return;
    await crud.handleEdit(crud.editTarget.id, {
      name: editForm.name,
      startsOn: editForm.startsOn || undefined,
      endsOn: editForm.endsOn || undefined,
      status: editForm.status,
    });
  }

  return (
    <>
      <PageShell
        title="Academic Years"
        subtitle={`${crud.total} academic year${crud.total !== 1 ? "s" : ""} registered`}
        onAdd={() => crud.setShowCreate(true)}
        addLabel="Add Year"
      >
        {crud.loading ? (
          <div className="space-y-2">
            {[1, 2].map((i) => <Skeleton key={i} className="h-14 w-full" />)}
          </div>
        ) : crud.items.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-10 text-center">
            <CalendarRange size={24} className="text-[var(--muted-foreground)] mb-2" />
            <p className="text-[13px] font-semibold text-[var(--foreground)]">No academic years yet</p>
            <p className="text-[11px] text-[var(--muted-foreground)] mt-1">Create your first academic year to get started.</p>
          </div>
        ) : (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] overflow-hidden">
            <div className="divide-y divide-[var(--border)]">
              {crud.items.map((ay) => (
                <div key={ay.id} className="flex items-center gap-3 px-3 py-3 hover:bg-[var(--muted)]/50 transition-colors">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--muted)] text-[var(--muted-foreground)]">
                    <CalendarRange size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{ay.name}</p>
                      <span className="rounded-md bg-[var(--accent)] px-1.5 py-0.5 text-[10px] text-[var(--muted-foreground)]">{ay.code}</span>
                    </div>
                    {(ay.startsOn || ay.endsOn) && (
                      <p className="text-[11px] text-[var(--muted-foreground)] mt-0.5">
                        {ay.startsOn ? new Date(ay.startsOn).toLocaleDateString() : "?"} - {ay.endsOn ? new Date(ay.endsOn).toLocaleDateString() : "?"}
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
                    { label: "Archive", icon: <Trash2 size={13} />, onClick: () => crud.setArchiveTarget(ay), variant: "danger" },
                  ]} />
                </div>
              ))}
            </div>
          </div>
        )}
      </PageShell>

      {/* Create Sheet */}
      <RightPullSheet open={crud.showCreate} title="Add Year" onClose={() => crud.setShowCreate(false)}>
        <form onSubmit={handleCreate} className="space-y-3">
          <InputField
            label="Code"
            value={createForm.code}
            onChange={(e) => setCreateForm({ ...createForm, code: e.target.value })}
            error={crud.fieldErrors.code}
            helperText="e.g. 2025-2026"
            prefix={<CalendarRange size={14} />}
          />
          <InputField
            label="Name"
            value={createForm.name}
            onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
            error={crud.fieldErrors.name}
            helperText="e.g. Tahun Ajaran 2025/2026"
          />
          <DatePicker
            label="Starts On"
            value={createForm.startsOn}
            onChange={(v) => setCreateForm({ ...createForm, startsOn: v })}
            error={crud.fieldErrors.startsOn}
          />
          <DatePicker
            label="Ends On"
            value={createForm.endsOn}
            onChange={(v) => setCreateForm({ ...createForm, endsOn: v })}
            error={crud.fieldErrors.endsOn}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => crud.setShowCreate(false)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">Cancel</button>
            <button type="submit" disabled={crud.creating} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {crud.creating && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" />}
              <Plus size={14} /> Create
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Edit Sheet */}
      <RightPullSheet open={!!crud.editTarget} title="Edit Year" onClose={() => crud.setEditTarget(null)}>
        <form onSubmit={handleEdit} className="space-y-3">
          <InputField
            label="Name"
            value={editForm.name}
            onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
            error={crud.fieldErrors.name}
            prefix={<CalendarRange size={14} />}
          />
          <DatePicker
            label="Starts On"
            value={editForm.startsOn}
            onChange={(v) => setEditForm({ ...editForm, startsOn: v })}
            error={crud.fieldErrors.startsOn}
          />
          <DatePicker
            label="Ends On"
            value={editForm.endsOn}
            onChange={(v) => setEditForm({ ...editForm, endsOn: v })}
            error={crud.fieldErrors.endsOn}
          />
          <SelectField
            label="Status"
            value={editForm.status}
            onChange={(val) => setEditForm({ ...editForm, status: val })}
            options={[
              { value: "active", label: "Active" },
              { value: "inactive", label: "Inactive" },
              { value: "archived", label: "Archived" },
            ]}
          />
          <div className="flex gap-2 justify-end pt-3">
            <button type="button" onClick={() => crud.setEditTarget(null)} className="h-8 px-3 rounded-lg text-[12px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--muted)] transition-colors">Cancel</button>
            <button type="submit" disabled={crud.editing} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-[var(--primary)] px-3 text-[12px] font-semibold text-[var(--primary-foreground)] shadow-sm hover:opacity-90 active:scale-[0.97] disabled:opacity-50 transition-all">
              {crud.editing ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" /> : "Save Changes"}
            </button>
          </div>
        </form>
      </RightPullSheet>

      {/* Archive Confirm */}
      <ConfirmDialog
        open={!!crud.archiveTarget}
        title="Archive Academic Year"
        description={`Are you sure you want to archive "${crud.archiveTarget?.name}"?`}
        confirmLabel="Archive"
        destructive
        loading={crud.archiving}
        onConfirm={() => crud.archiveTarget && crud.handleArchive(crud.archiveTarget.id)}
        onCancel={() => crud.setArchiveTarget(null)}
      />
    </>
  );
}
