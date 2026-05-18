"use client";

import { useState, useEffect } from "react";
import { useToast } from "@/components/ui/toast";
import { listStudents, archiveStudent, type Student } from "@/lib/modules-api";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/text-field";
import { Skeleton } from "@/components/ui/skeleton";
import { Search, BookOpen, Trash2 } from "lucide-react";
import { cn } from "@/lib/cn";

export default function StudentsPage() {
  const { toast } = useToast();
  const [students, setStudents] = useState<Student[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  async function load() {
    setLoading(true);
    const res = await listStudents({ search: search || undefined });
    if (res.data) {
      setStudents(res.data.data);
      setTotal(res.data.pagination.total);
    }
    setLoading(false);
  }

  useEffect(() => { load(); }, [search]);

  async function handleArchive(id: string) {
    const res = await archiveStudent(id);
    if (res.error) { toast({ tone: "error", title: "Failed", description: res.error.message }); return; }
    toast({ tone: "success", title: "Student archived" });
    load();
  }

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-[15px] font-bold text-[var(--foreground)] tracking-tight">Students</h2>
          <p className="text-[12px] text-[var(--muted-foreground)] mt-0.5">{total} student{total !== 1 ? "s" : ""}</p>
        </div>
      </div>

      <div className="max-w-xs">
        <TextField label="Search students..." value={search} onChange={(e) => setSearch(e.target.value)} prefix={<Search size={15} />} />
      </div>

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
              <div key={s.id} className="flex items-center gap-4 px-5 py-3.5 hover:bg-[var(--muted)]/50 transition-colors">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--brand-soft)] text-[var(--brand)]">
                  <BookOpen size={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium text-[var(--foreground)] truncate">{s.displayName}</p>
                  <p className="text-[11px] text-[var(--muted-foreground)]">{s.gradeLevel || s.email}</p>
                </div>
                {s.studentIdNumber && <span className="text-[10px] text-[var(--muted-foreground)] font-mono">{s.studentIdNumber}</span>}
                <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-medium", s.status === "active" ? "bg-[var(--success-soft)] text-[var(--success)]" : "bg-[var(--muted)] text-[var(--muted-foreground)]")}>{s.status}</span>
                <Button variant="ghost" size="sm" onClick={() => handleArchive(s.id)}><Trash2 size={13} /></Button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
