"use client";

import { InputField } from "@/components/ui/input-field";
import { SelectField } from "@/components/ui/select-field";
import { TextareaField } from "@/components/ui/textarea-field";

const COGNITIVE_OPTIONS = [
  { value: "", label: "— Pilih level —" },
  { value: "C1", label: "C1 · Mengingat" },
  { value: "C2", label: "C2 · Memahami" },
  { value: "C3", label: "C3 · Menerapkan" },
  { value: "C4", label: "C4 · Menganalisis" },
  { value: "C5", label: "C5 · Mengevaluasi" },
  { value: "C6", label: "C6 · Mencipta" },
];

const DIFFICULTY_OPTIONS = [
  { value: "", label: "— Pilih tingkat —" },
  { value: "mudah", label: "Mudah" },
  { value: "sedang", label: "Sedang" },
  { value: "sulit", label: "Sulit" },
];

const SEMESTER_OPTIONS = [
  { value: "", label: "— Pilih semester —" },
  { value: "Ganjil", label: "Ganjil" },
  { value: "Genap", label: "Genap" },
];

export interface MerdekaKisiKisiFieldsProps {
  cpReferenceId?: string;
  cpReferenceOptions?: Array<{ value: string; label: string }>;
  cpElementId?: string;
  cpElementOptions?: Array<{ value: string; label: string }>;
  capaianPembelajaran: string;
  elemenCp: string;
  tujuanPembelajaran: string;
  materiPokok: string;
  kelas: string;
  semester: string;
  cognitiveLevel: string;
  difficulty: string;
  indikatorSoal: string;
  onCPReferenceId?: (value: string) => void;
  onCPElementId?: (value: string) => void;
  onCapaianPembelajaran: (value: string) => void;
  onElemenCp: (value: string) => void;
  onTujuanPembelajaran: (value: string) => void;
  onMateriPokok: (value: string) => void;
  onKelas: (value: string) => void;
  onSemester: (value: string) => void;
  onCognitiveLevel: (value: string) => void;
  onDifficulty: (value: string) => void;
  onIndikatorSoal: (value: string) => void;
  errors?: Record<string, string>;
}

export function MerdekaKisiKisiFields({
  cpReferenceId = "",
  cpReferenceOptions = [],
  cpElementId = "",
  cpElementOptions = [],
  capaianPembelajaran,
  elemenCp,
  tujuanPembelajaran,
  materiPokok,
  kelas,
  semester,
  cognitiveLevel,
  difficulty,
  indikatorSoal,
  onCPReferenceId,
  onCPElementId,
  onCapaianPembelajaran,
  onElemenCp,
  onTujuanPembelajaran,
  onMateriPokok,
  onKelas,
  onSemester,
  onCognitiveLevel,
  onDifficulty,
  onIndikatorSoal,
  errors = {},
}: MerdekaKisiKisiFieldsProps) {
  return (
    <div className="space-y-3 rounded-xl border border-[var(--border)] bg-[var(--accent)]/40 p-3">
      <div>
        <p className="text-[12px] font-semibold text-[var(--foreground)]">Kisi-kisi Kurikulum Merdeka</p>
        <p className="mt-0.5 text-[11px] leading-relaxed text-[var(--muted-foreground)]">
          Gunakan CP dan Elemen CP. Jangan gunakan KD/SK. TP dan indikator harus selaras dengan level kognitif.
        </p>
      </div>

      {(onCPReferenceId || onCPElementId) && (
        <div className="grid gap-2 md:grid-cols-2">
          {onCPReferenceId && (
            <SelectField
              label="CP resmi"
              value={cpReferenceId}
              onChange={onCPReferenceId}
              options={[{ value: "", label: "— Pilih CP resmi —" }, ...cpReferenceOptions]}
              helperText="CP disalin verbatim dari master CP."
            />
          )}
          {onCPElementId && (
            <SelectField
              label="Elemen CP resmi"
              value={cpElementId}
              onChange={onCPElementId}
              options={[{ value: "", label: "— Pilih elemen —" }, ...cpElementOptions]}
              disabled={cpElementOptions.length === 0}
            />
          )}
        </div>
      )}

      <TextareaField
        label="Capaian Pembelajaran"
        value={capaianPembelajaran}
        onChange={(e) => onCapaianPembelajaran(e.target.value)}
        error={errors.capaianPembelajaran}
      />

      <InputField label="Elemen CP" value={elemenCp} onChange={(e) => onElemenCp(e.target.value)} error={errors.elemenCp} />
      <InputField label="Tujuan Pembelajaran" value={tujuanPembelajaran} onChange={(e) => onTujuanPembelajaran(e.target.value)} error={errors.tujuanPembelajaran} helperText="Format ideal: Peserta didik dapat [KKO] ... berdasarkan [kondisi] dengan [degree]." />
      <InputField label="Materi Pokok" value={materiPokok} onChange={(e) => onMateriPokok(e.target.value)} error={errors.materiPokok} />

      <div className="grid grid-cols-2 gap-2">
        <InputField label="Kelas" value={kelas} onChange={(e) => onKelas(e.target.value)} error={errors.kelas} />
        <SelectField label="Semester" value={semester} onChange={onSemester} options={SEMESTER_OPTIONS} />
      </div>

      <div className="grid grid-cols-2 gap-2">
        <SelectField label="Level Kognitif" value={cognitiveLevel} onChange={onCognitiveLevel} options={COGNITIVE_OPTIONS} />
        <SelectField label="Tingkat Kesulitan" value={difficulty} onChange={onDifficulty} options={DIFFICULTY_OPTIONS} />
      </div>

      <TextareaField
        label="Indikator Soal"
        value={indikatorSoal}
        onChange={(e) => onIndikatorSoal(e.target.value)}
        error={errors.indikatorSoal}
      />
    </div>
  );
}
