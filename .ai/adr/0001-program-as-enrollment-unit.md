# ADR-0001: Program as Enrollment Unit

**Date:** 2026-05-18
**Status:** Accepted
**Context:** Morfoschools LMS needs a way to group courses and exams into structured learning packages that can be assigned to students.

## Decision

Program is the primary enrollment unit. Students enroll to Programs, not to individual courses or exams.

## Structure

```
Program
├── Section (ordered, unlock_mode: sequential | always_open)
│   ├── Item (course | exam, ordered, sequential)
│   ├── Item
│   └── ...
├── Section
│   └── ...
└── ...
```

## Rules

1. Courses and Exams are standalone entities (can be created/edited independently)
2. Students access courses/exams ONLY through Program enrollment (v1)
3. Sections are ordered; unlock mode configurable per section
4. Items within a section are sequential and orderable
5. Each item has its own completion config (passing_grade, max_attempts, is_required)
6. Completion = all required items done (per their individual config)
7. max_attempts default = 1 (guru explicitly sets remedial allowance)
8. Course/Exam reusable across multiple Programs (reference, not copy)

## Alternatives Considered

1. **Direct enrollment to course/exam** — simpler but no cohesive progress tracking, no structured learning path, assignment overhead per item
2. **Certification model (LearnDash-style)** — naming doesn't fit Indonesian school context
3. **Flat list (no sections)** — insufficient for organizing multi-topic programs

## Consequences

- Assignment is one action (assign Program to class)
- Progress tracking is cohesive (per-program, per-section, per-item)
- Reporting is meaningful ("siapa yang sudah selesai Program X")
- Slightly more complex schema than direct enrollment
- Quick standalone quiz = Program with 1 section, 1 item
