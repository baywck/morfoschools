"use client";

/**
 * KisiKisiToggle (Phase 9.9 polish) — proper iOS-style toggle switch
 * for the exam header. Replaces the old single-pill button so users
 * see a tactile on/off control consistent with the rest of the design
 * system.
 *
 * Layout: "Kisi-Kisi" label on the left, switch on the right, with a
 * lock icon when the exam isn't draft (toggle becomes uneditable). On
 * turn-off the existing confirm dialog flow is preserved so users
 * can't accidentally drop coverage.
 */

import { useState } from "react";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ToggleSwitch } from "@/components/ui/toggle-switch";
import { Lock } from "lucide-react";
import type { Exam } from "@/lib/modules-api";

export interface KisiKisiToggleProps {
  exam: Exam;
  onToggle: (enabled: boolean) => Promise<void>;
}

export function KisiKisiToggle({ exam, onToggle }: KisiKisiToggleProps) {
  const [confirmOff, setConfirmOff] = useState(false);
  const [busy, setBusy] = useState(false);
  const isDraft = exam.status === "draft";
  const enabled = exam.usesKisiKisi;
  const disabled = !isDraft || busy;

  async function handleChange(next: boolean) {
    if (disabled) return;
    if (!next) {
      setConfirmOff(true);
      return;
    }
    setBusy(true);
    try {
      await onToggle(true);
    } finally {
      setBusy(false);
    }
  }

  async function handleConfirmOff() {
    setBusy(true);
    try {
      await onToggle(false);
    } finally {
      setBusy(false);
      setConfirmOff(false);
    }
  }

  const tooltip = isDraft
    ? enabled
      ? "Matikan tracking kisi-kisi"
      : "Aktifkan tracking kisi-kisi (slot per soal otomatis)"
    : "Toggle hanya bisa diubah saat draft";

  return (
    <>
      <div
        className="inline-flex h-8 items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--background)] px-2.5"
        title={tooltip}
      >
        <ToggleSwitch
          checked={enabled}
          onChange={handleChange}
          label="Kisi-Kisi"
          disabled={disabled}
          ariaLabel={enabled ? "Kisi-kisi aktif" : "Kisi-kisi nonaktif"}
          trailing={
            !isDraft ? (
              <Lock size={11} className="text-[var(--muted-foreground)]" />
            ) : null
          }
        />
      </div>

      <ConfirmDialog
        open={confirmOff}
        title="Matikan kisi-kisi?"
        description="Slot dan binding tetap disimpan, tapi enforcement (publish gate coverage) akan dinonaktifkan. Bisa diaktifkan lagi nanti tanpa kehilangan data."
        confirmLabel="Matikan"
        loading={busy}
        onConfirm={handleConfirmOff}
        onCancel={() => setConfirmOff(false)}
      />
    </>
  );
}
