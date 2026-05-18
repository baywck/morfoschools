"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { listStaff, archiveStaff, type Staff } from "@/lib/modules-api";
import { Button } from "@/components/ui/button";
import { PageShell } from "@/components/layout/page-shell";
import { RowActions } from "@/components/ui/row-actions";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Briefcase, Trash2 } from "lucide-react";
import { cn } from "@/lib/cn";

export default function StaffPage() {
  const { toast } = useToast();
  const [staff, setStaff] = useState<Staff[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
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

  async function handleArchive(id: string) {
    const res = await archiveStaff(id);
    if (res.error) { toast({ tone: "error", title: "Failed", description: res.error.message }); return; }
    toast({ tone: "success", title: "Staff archived" });
    setStaffToArchive(null);
    load();
  }

  return (
    <PageShell
      title="Staff"
      subtitle={`${total} staff member${total !== 1 ? "s" : ""}`}
      search={{ value: search, onChange: setSearch }}
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
                    { label: "Archive", icon: <Trash2 size={14} />, onClick: () => setStaffToArchive(s), variant: "danger" }
                  ]}
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </PageShell>
  );
}
