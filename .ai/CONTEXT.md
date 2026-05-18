# Domain Glossary — Morfoschools

Canonical vocabulary. All code, UI labels, and docs must use these terms consistently.

## Core Entities

| Term | Definition |
|------|-----------|
| **Tenant** | Satu sekolah/institusi. Semua domain data tenant-scoped. |
| **Program** | Container utama yang menggabungkan courses + exams jadi satu paket terstruktur. Siswa enroll ke Program, bukan ke course/exam langsung. UI label: "Program Belajar". |
| **Section** | Grouping di dalam Program. Ordered. Unlock mode: sequential atau always_open. |
| **Program Item** | Satu entry di dalam Section. Bisa type `course` atau `exam`. Ordered, sequential dalam section. |
| **Course** | Entity mandiri — materi pembelajaran (modules, lessons, resources). Diakses siswa hanya via Program enrollment. |
| **Exam** | Entity mandiri — ujian/assessment. Diakses siswa hanya via Program enrollment. |
| **Enrollment** | Relasi student → Program. Satu enrollment per student per program. Progress tracked di sini. |
| **Class Assignment** | Relasi temporal enrollment → class_section. Bisa berubah (pindah kelas) tanpa menghilangkan enrollment/progress. |
| **Program Assignment** | Target assignment Program ke class_section atau individual student. Trigger auto-enroll. |

## Progress & Scoring

| Term | Definition |
|------|-----------|
| **Item Progress** | Status + result per program_item per enrollment. Resolve by item_id, bukan posisi. |
| **Status (progress)** | Access state: `locked`, `available`, `in_progress`, `completed` |
| **Result (progress)** | Academic outcome: `none`, `passed`, `failed`, `exempted`, `overridden` |
| **Enrollment Status** | Administrative lifecycle: `draft`, `active`, `suspended`, `completed`, `withdrawn`, `cancelled` |
| **Enrollment Result** | Overall outcome: `not_started`, `in_progress`, `pending_review`, `passed`, `failed`, `incomplete`, `void` |
| **Score Snapshot** | Immutable record saat grading. Menyimpan max_score, passing_score saat itu. Tidak terpengaruh edit exam. |
| **Attempt** | Satu percobaan mengerjakan exam. Default max 1, configurable per item. |
| **Passing Grade** | Minimum score per item untuk result = passed. Configurable per program_item. |
| **is_required** | Item wajib untuk completion. Optional items tidak dihitung. |

## Academic Structure

| Term | Definition |
|------|-----------|
| **Academic Year** | Tahun ajaran (e.g. 2025-2026). Tenant-scoped. |
| **Semester** | Periode dalam tahun ajaran. Ganjil/Genap. |
| **Class Section** | Kelas administratif (e.g. X-IPA-1). Tenant-scoped. |
| **Subject** | Mata pelajaran. |
| **Subject Group** | Rombongan belajar / peminatan. |
| **Teaching Assignment** | Relasi guru → subject → class_section. |

## Roles

| Term | Definition |
|------|-----------|
| **Master Admin** | Platform-level. Kelola semua tenant. Tidak punya default tenant. |
| **School Admin** | Admin satu tenant/sekolah. |
| **Academic Admin** | Operasional akademik (kelas, mapel, jadwal). |
| **Teacher** | Buat course, kelola exam, grading. |
| **Student** | Akses program, ikut exam, lihat progress. |
| **Staff** | Operasional terbatas. |
| **Guardian** | Lihat data anak. |

## Key Rules

- **Program-only access (v1):** Siswa hanya bisa akses course/exam melalui Program enrollment.
- **Score decoupled:** Score immutable, snapshot at grade time. Edit exam tidak rusak history.
- **Progress by item_id:** Reorder/restructure Program tidak hilangkan progress. Auto-complete jika item sudah pernah passed.
- **No locking:** Guru bebas restructure Program. Completion hitung items aktif saja.
- **Hapus item:** Progress jadi orphan, tidak dihitung ke completion.
- **Reset progress:** Explicit guru action untuk paksa siswa ulang.
- **Pindah kelas:** Enrollment tetap active. Class assignment temporal.
- **Remedial:** max_attempts configurable per item. Default 1x.
