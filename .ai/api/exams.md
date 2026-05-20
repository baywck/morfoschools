# Exams API

> Permission: `exams:read` / `exams:write` | Tenant-scoped
> Teachers can only author exams for subjects in their `teacher_subjects`
> assignments. Tenant admins are unrestricted.

## Status lifecycle

`draft` → `published` (irreversible without unpublish, which is not yet
exposed) → `archived` ↔ `draft` (via restore).

Publishing requires at least one question.

## GET /api/v1/exams

**Query:** `?search=&status=&subjectId=&page=1&pageSize=20`

**200:**
```json
{
  "data": [{
    "id", "title", "description", "subjectId", "subjectName",
    "examType", "durationMinutes", "maxScore", "passingScore",
    "status", "shuffleQuestions", "shuffleOptions", "showResultImmediately",
    "publishedAt", "createdAt", "questionCount", "totalPoints"
  }],
  "pagination": { "page", "pageSize", "total", "totalPages" }
}
```

## GET /api/v1/exams/{id}
Same shape as a list row. 404 when not found in tenant.

## POST /api/v1/exams

**Body:**
```json
{
  "title": "required",
  "description?",
  "subjectId?": "uuid",
  "examType?": "quiz|midterm|final|tryout|daily",
  "durationMinutes?": 60,
  "maxScore?": 100,
  "passingScore?": 70,
  "shuffleQuestions?": false,
  "shuffleOptions?": false,
  "showResultImmediately?": false
}
```

**201:** `{ "id", "status": "draft" }`

## PATCH /api/v1/exams/{id}
All fields optional. Subject change re-runs RBAC check.

## PATCH /api/v1/exams/{id}/publish
**200:** `{ "id", "status": "published" }`
**409:** when exam is not in `draft` status.
**422 (validation):** `{ "fields": { "questions": "..." } }` when zero questions.

## PATCH /api/v1/exams/{id}/archive
**200:** `{ "id", "status": "archived" }`

## PATCH /api/v1/exams/{id}/restore
Restores an archived exam back to `draft`.
**200:** `{ "id", "status": "draft" }`

---

## Sections

### GET /api/v1/exams/{id}/sections
**200:** `{ "data": [{ "id", "examId", "title", "description", "sortOrder", "questionCount", "createdAt" }] }`

### POST /api/v1/exams/{id}/sections
**Body:** `{ "title", "description?", "sortOrder?": "auto = max+1" }`

### PATCH /api/v1/exam-sections/{sectionId}
### DELETE /api/v1/exam-sections/{sectionId}
Deleting a section sets `section_id = NULL` on its questions (they are not deleted).

---

## Questions

### Question types
- `multiple_choice` — 2 to 10 options, 1+ correct
- `true_false` — exactly 2 options, exactly 1 correct
- `short_answer` — free-text, optional `correctAnswer` for grading hint
- `essay` — free-text, optional `rubric` JSONB

### Scoring modes (multiple_choice only)

| Mode | Behavior |
|------|----------|
| `correct_all` (default) | Must select EXACTLY all correct options for full points |
| `correct_one` | Selecting any single correct option scores full points |
| `percentage` | `score = points * (correct_selected / total_correct)`. Optional `wrongPenaltyPct` (0..1) subtracts `points * wrongPenaltyPct` per wrong selection (clamped ≥0). Optional per-option `pointsWeight` overrides equal-share when set. |

### Shuffle resolution

If a question has `shuffleOptionsOverride` set, it wins over
`exam.shuffleOptions`. Otherwise the exam-level flag applies.

### GET /api/v1/exams/{id}/questions

**200:**
```json
{ "data": [{
    "id", "examId", "sectionId", "questionType", "content", "explanation",
    "correctAnswer?", "rubric?", "points", "sortOrder", "scoringMode",
    "wrongPenaltyPct?", "shuffleOptionsOverride?", "correctCount",
    "options": [{ "id", "content", "isCorrect", "sortOrder", "pointsWeight?" }]
}] }
```

### POST /api/v1/exams/{id}/questions
**Body:** mirror of the read shape; `options` is optional and only used for
MCQ / true/false.

Pre-create dedup: a question whose normalized content already exists in
the same exam returns `422` with `fields.content`.

### PATCH /api/v1/questions/{questionId}
### DELETE /api/v1/questions/{questionId}
### POST /api/v1/questions/{questionId}/options
### PATCH /api/v1/options/{optionId}
### DELETE /api/v1/options/{optionId}

The `is_correct` count on the parent question is maintained automatically
by a trigger; clients do not need to recount.

---

## Gate Windows (Schedule)

A `published` exam without an active gate window is considered closed.
Multiple windows per exam are supported (e.g. retake window).

### GET /api/v1/exams/{id}/gates
**200:** `{ "data": [{ "id", "examId", "opensAt", "closesAt", "accessCode?", "isOpen", "createdAt" }] }`

### POST /api/v1/exams/{id}/gates
**Body:** `{ "opensAt": "ISO 8601", "closesAt": "ISO 8601", "accessCode?": "string" }`

### PATCH /api/v1/exam-gates/{gateId}
### DELETE /api/v1/exam-gates/{gateId}

---

## AI Tools

The chatbot exposes seven capabilities under the `exams` domain:

| Tool | Description |
|------|------|
| `list_exams` | Browse exams in active tenant |
| `get_exam` | Single-exam summary |
| `list_questions` | Used by the bot before batch-create to avoid duplicates |
| `create_exam` | Proposes a new exam (user confirms) |
| `create_question` | Proposes a single question with options |
| `batch_create_questions` | Proposes many at once; all-or-nothing validation with per-item failures returned for retry |

All write capabilities funnel through pre-propose duplicate guards
(see `.ai/standards/ai-tool-guards.md`).
