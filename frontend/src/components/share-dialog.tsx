"use client";

import { useEffect, useRef, useState } from "react";
import {
  listCollaborators,
  inviteCollaborator,
  updateCollaboratorRole,
  removeCollaborator,
  transferOwnership,
  listUsers,
  type Collaborator,
  type CollabResource,
  type CollaboratorRole,
  type User,
} from "@/lib/modules-api";
import { useToast } from "@/components/ui/toast";
import { useAuth } from "@/lib/auth-provider";
import { RightPullSheet } from "@/components/ui/right-pull-sheet";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { SelectField } from "@/components/ui/select-field";
import { SearchInput } from "@/components/ui/search-input";
import {
  Crown,
  X as XIcon,
  UserPlus,
  ArrowRightLeft,
  Loader2,
  ChevronDown,
  Check,
} from "lucide-react";
import { cn } from "@/lib/cn";

/**
 * ShareDialog — reusable invite/role/transfer panel for any resource that
 * supports the Phase 9.5 collaboration model (exams, courses,
 * blueprint-templates).
 *
 * Designed to slot inside an existing layout via RightPullSheet. Caller
 * controls open/close; this component owns the data + actions.
 *
 * Permission semantics:
 *   - Anyone with read access can see the list
 *   - Only owner / tenant admin can invite, change role, remove, transfer
 *     (the backend enforces; we just dim the UI to match)
 */
export function ShareDialog({
  open,
  onClose,
  resource,
  resourceId,
  resourceName,
  currentUserCanManage,
}: {
  open: boolean;
  onClose: () => void;
  resource: CollabResource;
  resourceId: string;
  resourceName: string;
  /** Caller computes from the parent resource (owner_user_id or tenant
   *  admin). When false, only read-only view is rendered. */
  currentUserCanManage: boolean;
}) {
  const { toast } = useToast();
  const { session } = useAuth();
  const [loading, setLoading] = useState(true);
  const [owner, setOwner] = useState<Collaborator | null>(null);
  const [collabs, setCollabs] = useState<Collaborator[]>([]);

  // User search state for invite picker
  const [search, setSearch] = useState("");
  const [searchResults, setSearchResults] = useState<User[]>([]);
  const [searching, setSearching] = useState(false);
  const [inviteRole, setInviteRole] = useState<CollaboratorRole>("editor");
  const [inviting, setInviting] = useState(false);

  // Confirm states
  const [removeTarget, setRemoveTarget] = useState<Collaborator | null>(null);
  const [transferTarget, setTransferTarget] = useState<Collaborator | null>(null);
  const [working, setWorking] = useState(false);

  async function reload() {
    if (!resourceId) return;
    setLoading(true);
    const res = await listCollaborators(resource, resourceId);
    if (res.data) {
      setOwner(res.data.owner ?? null);
      // Backend returns { owner, collaborators }. Older payloads or a
      // legacy {data: []} shape would lack `collaborators` — default
      // to [] so the .length read in render never crashes.
      setCollabs(
        Array.isArray(res.data.collaborators)
          ? res.data.collaborators
          : [],
      );
    }
    setLoading(false);
  }

  useEffect(() => {
    if (open) reload();
  }, [open, resourceId]);

  // Debounced user search
  useEffect(() => {
    if (!open || !currentUserCanManage) return;
    if (search.trim().length < 2) {
      setSearchResults([]);
      return;
    }
    const handle = setTimeout(async () => {
      setSearching(true);
      const res = await listUsers({ search });
      setSearching(false);
      if (res.data) {
        // Exclude already-collaborators and the owner
        const existingIds = new Set([
          owner?.userId,
          ...collabs.map((c) => c.userId),
          session?.user.id,
        ]);
        setSearchResults(
          res.data.data.filter((u) => !existingIds.has(u.id) && u.status !== "archived"),
        );
      }
    }, 250);
    return () => clearTimeout(handle);
  }, [search, open, currentUserCanManage, owner, collabs, session]);

  async function handleInvite(user: User) {
    setInviting(true);
    const res = await inviteCollaborator(resource, resourceId, {
      userId: user.id,
      role: inviteRole,
    });
    setInviting(false);
    if (res.error) {
      toast({ tone: "error", title: "Invite failed", description: res.error.message });
      return;
    }
    toast({
      tone: "success",
      title: `Invited ${user.displayName}`,
      description: `Role: ${inviteRole}`,
    });
    setSearch("");
    setSearchResults([]);
    reload();
  }

  async function handleRoleChange(c: Collaborator, role: CollaboratorRole) {
    const res = await updateCollaboratorRole(resource, resourceId, c.id, role);
    if (res.error) {
      toast({ tone: "error", title: "Update failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: `Role updated to ${role}` });
    reload();
  }

  async function handleRemove() {
    if (!removeTarget) return;
    setWorking(true);
    const res = await removeCollaborator(resource, resourceId, removeTarget.id);
    setWorking(false);
    if (res.error) {
      toast({ tone: "error", title: "Remove failed", description: res.error.message });
      return;
    }
    toast({ tone: "success", title: `Removed ${removeTarget.displayName}` });
    setRemoveTarget(null);
    reload();
  }

  async function handleTransfer() {
    if (!transferTarget) return;
    setWorking(true);
    const res = await transferOwnership(resource, resourceId, transferTarget.userId);
    setWorking(false);
    if (res.error) {
      toast({ tone: "error", title: "Transfer failed", description: res.error.message });
      return;
    }
    toast({
      tone: "success",
      title: "Ownership transferred",
      description: `${transferTarget.displayName} is now the owner.`,
    });
    setTransferTarget(null);
    reload();
  }

  return (
    <>
      <RightPullSheet open={open} title={`Collaborator — ${resourceName}`} onClose={onClose}>
        {loading ? (
          <div className="flex items-center justify-center py-8 text-[var(--muted-foreground)]">
            <Loader2 size={16} className="animate-spin" />
          </div>
        ) : (
          <div className="space-y-5">
            {/* Owner */}
            <div>
              <p className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">
                Owner
              </p>
              {owner ? (
                <div className="flex items-center gap-3 rounded-lg border border-[var(--border)] bg-[var(--accent)] px-3 py-2">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--brand-soft)] text-[var(--brand)]">
                    <Crown size={14} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-[12px] font-medium text-[var(--foreground)] truncate">
                      {owner.displayName}
                    </p>
                    <p className="text-[10px] text-[var(--muted-foreground)] truncate">
                      {owner.email}
                    </p>
                  </div>
                  <span className="rounded-md bg-[var(--brand-soft)] px-2 py-0.5 text-[10px] font-medium text-[var(--brand)]">
                    owner
                  </span>
                </div>
              ) : (
                <p className="text-[11px] text-[var(--muted-foreground)]">
                  No owner assigned
                </p>
              )}
            </div>

            {/* Collaborators */}
            <div>
              <p className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">
                Collaborators ({collabs.length})
              </p>
              {collabs.length === 0 ? (
                <p className="rounded-lg border border-dashed border-[var(--border-strong)] bg-[var(--accent)] p-3 text-[11px] text-[var(--muted-foreground)]">
                  No collaborators yet. Invite teachers below.
                </p>
              ) : (
                <div className="space-y-2">
                  {collabs.map((c) => (
                    <div
                      key={c.id}
                      className="flex items-center gap-3 rounded-lg border border-[var(--border)] bg-[var(--card)] px-3 py-2"
                    >
                      <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--muted)] text-[var(--muted-foreground)]">
                        {c.displayName.charAt(0).toUpperCase()}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-[12px] font-medium text-[var(--foreground)] truncate">
                          {c.displayName}
                        </p>
                        <p className="text-[10px] text-[var(--muted-foreground)] truncate">
                          {c.email}
                        </p>
                      </div>
                      {currentUserCanManage ? (
                        <div className="flex items-center gap-1.5">
                          {/* Inline custom role pill — hand-rolled to
                              match the icon button height (h-7) and the
                              tokenized look of SelectField, without the
                              floating label or browser-default select
                              chrome. */}
                          <RolePill
                            value={c.role as CollaboratorRole}
                            onChange={(role) => handleRoleChange(c, role)}
                          />
                          <button
                            type="button"
                            onClick={() => setTransferTarget(c)}
                            title="Transfer ownership"
                            className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] hover:text-[var(--brand)]"
                          >
                            <ArrowRightLeft size={12} />
                          </button>
                          <button
                            type="button"
                            onClick={() => setRemoveTarget(c)}
                            title="Remove"
                            className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] bg-[var(--background)] text-[var(--muted-foreground)] hover:text-[var(--danger)]"
                          >
                            <XIcon size={12} />
                          </button>
                        </div>
                      ) : (
                        <span className="rounded-md bg-[var(--muted)] px-2 py-0.5 text-[10px] font-medium text-[var(--muted-foreground)]">
                          {c.role}
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Invite */}
            {currentUserCanManage && (
              <div>
                <p className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-[var(--muted-foreground)]">
                  Invite
                </p>
                <div className="space-y-2">
                  <SelectField
                    label="Role for new collaborator"
                    value={inviteRole}
                    onChange={(v) => setInviteRole(v as CollaboratorRole)}
                    options={[
                      { value: "editor", label: "Editor (can modify)" },
                      { value: "viewer", label: "Viewer (read-only)" },
                    ]}
                  />
                  <div>
                    <p className="mb-1 text-[11px] font-medium text-[var(--muted-foreground)]">
                      Find user
                    </p>
                    <SearchInput
                      value={search}
                      onChange={setSearch}
                      placeholder="Cari nama atau email (min 2 karakter)"
                    />
                  </div>
                  {searching && (
                    <p className="text-[11px] text-[var(--muted-foreground)]">
                      Searching...
                    </p>
                  )}
                  {!searching && search.length >= 2 && searchResults.length === 0 && (
                    <p className="text-[11px] text-[var(--muted-foreground)]">
                      Tidak ada user yang cocok (atau sudah jadi collaborator).
                    </p>
                  )}
                  {searchResults.length > 0 && (
                    <div className="max-h-60 overflow-y-auto rounded-lg border border-[var(--border)] bg-[var(--card)]">
                      {searchResults.slice(0, 8).map((u) => (
                        <button
                          key={u.id}
                          type="button"
                          disabled={inviting}
                          onClick={() => handleInvite(u)}
                          className={cn(
                            "flex w-full items-center gap-2 px-3 py-2 text-left text-[12px] hover:bg-[var(--muted)]",
                            "border-b border-[var(--border)] last:border-b-0",
                            "disabled:opacity-50 disabled:cursor-not-allowed",
                          )}
                        >
                          <UserPlus size={12} className="text-[var(--brand)]" />
                          <div className="flex-1 min-w-0">
                            <p className="font-medium text-[var(--foreground)] truncate">
                              {u.displayName}
                            </p>
                            <p className="text-[10px] text-[var(--muted-foreground)] truncate">
                              {u.email}
                            </p>
                          </div>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </RightPullSheet>

      <ConfirmDialog
        open={!!removeTarget}
        title="Remove collaborator?"
        description={`${removeTarget?.displayName} akan kehilangan akses. Bisa di-invite kembali kapan saja.`}
        confirmLabel="Remove"
        destructive
        loading={working}
        onConfirm={handleRemove}
        onCancel={() => setRemoveTarget(null)}
      />

      <ConfirmDialog
        open={!!transferTarget}
        title="Transfer ownership?"
        description={`Ownership akan dipindah ke ${transferTarget?.displayName}. Kamu akan jadi editor (tidak bisa lagi invite/transfer/archive). Lanjut?`}
        confirmLabel="Transfer"
        destructive
        loading={working}
        onConfirm={handleTransfer}
        onCancel={() => setTransferTarget(null)}
      />
    </>
  );
}

/**
 * RolePill — inline custom dropdown for collaborator role.
 *
 * Lives next to icon buttons in the row, so it has to:
 *   - match the h-7 visual height
 *   - avoid the browser-default <select> chrome
 *   - close on outside click and Escape
 *   - share token-driven palette with the rest of the dialog
 */
function RolePill({
  value,
  onChange,
}: {
  value: CollaboratorRole | "owner";
  onChange: (role: CollaboratorRole) => void;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    if (open) {
      document.addEventListener("mousedown", handleClick);
      document.addEventListener("keydown", handleKey);
    }
    return () => {
      document.removeEventListener("mousedown", handleClick);
      document.removeEventListener("keydown", handleKey);
    };
  }, [open]);

  const options: { value: CollaboratorRole; label: string }[] = [
    { value: "editor", label: "Editor" },
    { value: "viewer", label: "Viewer" },
  ];
  const selected = options.find((o) => o.value === value);
  const display = selected?.label ?? value;

  function pick(role: CollaboratorRole) {
    setOpen(false);
    if (role !== value) onChange(role);
  }

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "flex h-7 items-center gap-1 rounded-md border bg-[var(--background)] pl-2.5 pr-1.5 text-[11px] font-medium text-[var(--foreground)] transition-colors",
          open
            ? "border-[var(--field-focus)] ring-2 ring-[var(--field-ring)]"
            : "border-[var(--border)] hover:bg-[var(--muted)]",
        )}
      >
        <span>{display}</span>
        <ChevronDown
          size={12}
          className={cn(
            "text-[var(--muted-foreground)] transition-transform",
            open && "rotate-180",
          )}
        />
      </button>
      {open && (
        <div className="absolute right-0 z-30 mt-1 w-32 overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--card)] p-1 shadow-lg">
          {options.map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => pick(opt.value)}
              className={cn(
                "flex w-full items-center justify-between rounded-md px-2.5 py-1.5 text-[12px] transition-colors",
                opt.value === value
                  ? "bg-[var(--muted)] text-[var(--foreground)] font-medium"
                  : "text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]",
              )}
            >
              <span>{opt.label}</span>
              {opt.value === value && (
                <Check size={12} className="text-[var(--brand)]" />
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
