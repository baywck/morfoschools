# Exam Authoring AI Agent Architecture

## Goal

Build robust AI agents for exam discussion, question creation, stimulus groups, and Kurikulum Merdeka kisi-kisi/blueprints without reintroducing generic tool-loop risks.

## Non-Negotiable Architecture

All writes follow:

```text
active exam context -> LLM intent extraction -> backend validation -> proposal -> explicit confirmation -> transaction
```

Discussion answers are LLM-only and never mutate data. The agent must not use deterministic template fallback questions, fake confirmations, stale global chat context, or LLM-selected database mutations.

## Curriculum Context Hydration

When an exam has `usesKisiKisi=true`, Questions Manager and agent workflows should first resolve curriculum context:

```text
load CP master from local DB -> if missing, attempt Kemendikbuddasmen fetch -> save normalized CP locally -> if still unavailable, return warning not blocking error
```

If CP is unavailable, AI may produce draft TP/indicator only with explicit warning. It must not claim official CP unless backed by local/fetched source.

## Kurikulum Merdeka Blueprint Rules

Kisi-kisi must use CP and TP, not KD/SK. Required slot fields:

- Capaian Pembelajaran
- Elemen CP
- Tujuan Pembelajaran
- Materi Pokok
- Kelas/Semester where available
- Cognitive Level C1-C6
- Indikator Soal with explicit stimulus
- Bentuk Soal
- Nomor Soal

Indicator format should be: `Disajikan [stimulus], peserta didik dapat [KKO] ...`.

The cognitive level, KKO in TP, and KKO in indicator must align.

## Question Quality Gate

Generated questions are reviewed against seven criteria:

1. Valid against target competency, TP, indicator, and cognitive level.
2. HOTS and contextual for C4-C6.
3. HOTS questions must have stimulus.
4. Fair and unbiased.
5. Single-answer MCQ has exactly one defensible correct answer and plausible distractors.
6. Clear standard Indonesian, no ambiguity or double negatives.
7. Independent from other questions; remains valid when shuffled.

Hard blocking examples:

- MCQ has zero or multiple correct answers.
- Duplicate MCQ options.
- C4-C6 question without stimulus.
- Slot/question type mismatch.
- KD/SK appears in Kurikulum Merdeka blueprint.
- Indicator lacks stimulus.
- Question refers to previous/other question.

Warnings examples:

- CP source unavailable/unverified.
- TP KKO does not clearly match level.
- Indicator KKO does not clearly match level.
- Wording too long.
- Possible weak distractor or ambiguity.

## Preferred Authoring Flow

If kisi-kisi is active:

```text
blueprint slot -> one generated question -> quality gate -> proposal -> confirm
```

One indicator should map to exactly one question. Batch generation should prefer creating/using slots first, then generating one question per slot.

If kisi-kisi is inactive:

```text
exam context + explicit topic -> question draft -> quality gate -> proposal -> confirm
```

## First Implementation Slices

1. Foundation:
   - `agent_curriculum_rules.go`
   - `agent_question_quality.go`
   - tests for hard gates
2. Curriculum context hydration endpoint.
3. `create_question_from_slot` workflow.
4. `create_blueprint_slots` workflow.
5. Batch question generation with chunking and aggregate validation.
