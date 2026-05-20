"use client";

/**
 * KisiKisiFields — shared inline form for kisi-kisi metadata.
 *
 * Used by both the question accordion (Phase 9.9 inline editor) and
 * the blueprint slot create/edit sheet (Phase 9.10 unification). Same
 * layout, same control widths, same textarea for indikator. Callers
 * pass the value/setter pair for every field.
 *
 * The component is intentionally pure — no labels for blueprint-only
 * fields like competencyDescription, questionType, or points; those
 * live outside the kisi-kisi axis and are rendered by the caller.
 */

import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { TextareaField } from "@/components/ui/textarea-field";
import { cn } from "@/lib/cn";

export const COGNITIVE_OPTIONS = [
  { value: "", label: "—" },
  { value: "C1", label: "C1 — Mengingat" },
  { value: "C2", label: "C2 — Memahami" },
  { value: "C3", label: "C3 — Menerapkan" },
  { value: "C4", label: "C4 — Menganalisis" },
  { value: "C5", label: "C5 — Mengevaluasi" },
  { value: "C6", label: "C6 — Mencipta" },
];

export const DIFFICULTY_OPTIONS = [
  { value: "", label: "—" },
  { value: "mudah", label: "Mudah" },
  { value: "sedang", label: "Sedang" },
  { value: "sulit", label: "Sulit" },
];

export const AKM_LEVEL_OPTIONS = [
  { value: "", label: "—" },
  ...[1, 2, 3, 4, 5].map((n) => ({ value: String(n), label: `Level ${n}` })),
];

export interface KisiKisiFieldsProps {
  isAkm: boolean;
  /** Render fields as disabled (template-locked or non-editor). */
  readOnly?: boolean;

  competency: string;
  competencyDescription: string;
  materi: string;
  indikator: string;
  cognitive: string;
  difficulty: string;
  akmKonten: string;
  akmKonteks: string;
  akmProses: string;
  akmLevel: string;

  onCompetency: (v: string) => void;
  onCompetencyDescription: (v: string) => void;
  onMateri: (v: string) => void;
  onIndikator: (v: string) => void;
  onCognitive: (v: string) => void;
  onDifficulty: (v: string) => void;
  onAkmKonten: (v: string) => void;
  onAkmKonteks: (v: string) => void;
  onAkmProses: (v: string) => void;
  onAkmLevel: (v: string) => void;
}

export function KisiKisiFields({
  isAkm,
  readOnly = false,
  competency,
  competencyDescription,
  materi,
  indikator,
  cognitive,
  difficulty,
  akmKonten,
  akmKonteks,
  akmProses,
  akmLevel,
  onCompetency,
  onCompetencyDescription,
  onMateri,
  onIndikator,
  onCognitive,
  onDifficulty,
  onAkmKonten,
  onAkmKonteks,
  onAkmProses,
  onAkmLevel,
}: KisiKisiFieldsProps) {
  return (
    <div className="space-y-2">
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <InputField
          label={isAkm ? "Kompetensi (opsional)" : "KD / Kompetensi"}
          value={competency}
          onChange={(e) => onCompetency(e.target.value)}
          disabled={readOnly}
        />
        <InputField
          label="Materi"
          value={materi}
          onChange={(e) => onMateri(e.target.value)}
          disabled={readOnly}
        />
      </div>
      <InputField
        label="Deskripsi kompetensi (opsional)"
        value={competencyDescription}
        onChange={(e) => onCompetencyDescription(e.target.value)}
        disabled={readOnly}
      />
      <div>
        <TextareaField
          label="Indikator"
          value={indikator}
          onChange={(e) => onIndikator(e.target.value)}
          rows={2}
          disabled={readOnly}
        />
      </div>
      {isAkm ? (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <InputField
            label="Konten"
            value={akmKonten}
            onChange={(e) => onAkmKonten(e.target.value)}
            disabled={readOnly}
          />
          <InputField
            label="Konteks"
            value={akmKonteks}
            onChange={(e) => onAkmKonteks(e.target.value)}
            disabled={readOnly}
          />
          <InputField
            label="Proses Kognitif"
            value={akmProses}
            onChange={(e) => onAkmProses(e.target.value)}
            disabled={readOnly}
          />
          <SelectField
            label="Level (1–5)"
            value={akmLevel}
            onChange={onAkmLevel}
            options={AKM_LEVEL_OPTIONS}
            disabled={readOnly}
          />
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <SelectField
            label="Cognitive Level"
            value={cognitive}
            onChange={onCognitive}
            options={COGNITIVE_OPTIONS}
            disabled={readOnly}
          />
          <SelectField
            label="Difficulty"
            value={difficulty}
            onChange={onDifficulty}
            options={DIFFICULTY_OPTIONS}
            disabled={readOnly}
          />
        </div>
      )}
    </div>
  );
}
