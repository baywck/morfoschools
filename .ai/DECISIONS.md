# Lightweight Decisions

## 2026-05-25 — CP master vs kisi-kisi grade scope

- CP master data is official/canonical by phase (A-F) and subject from Kemendikdasmen.
- Kisi-kisi templates are teacher/exam artifacts and must be per class/grade, not just per phase.
- Tenant available grades are inferred from active `class_sections.grade_level` for now; a future tenant setting may formalize grade range.
- CP seed/import UI should seed only phases relevant to tenant grade range by default. It should not blindly fetch A-F for SMA tenants.
- Blueprints UI is finalized for Kurikulum Merdeka only: CP, Elemen CP, TP, Materi Pokok, Kelas/Semester, Level Kognitif, Indikator Soal, Bentuk Soal/Nomor via position.
- K13/AKM DB compatibility remains for old data, but new blueprint templates should use `curriculumCode=merdeka` and `blueprintType=reguler`.
