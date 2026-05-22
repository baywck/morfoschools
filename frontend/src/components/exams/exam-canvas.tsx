"use client";

/**
 * ExamCanvas (Phase 9.8 rewrite) \u2014 section-mandatory authoring surface.
 *
 * Layout:
 *
 *   for each section:
 *     <SectionCard isOnlySection={sections.length === 1}>
 *       [groups + standalone questions in this section, sortable]
 *       <Footer: + Tambah Soal | + Tambah Group>
 *     </SectionCard>
 *   <AddSectionButton fullWidth dashedBorder />
 *
 * The old global toolbar (+Section / +Group / +Soal) is removed: every
 * add-action lives inside the section that will own the new item.
 *
 * Kisi-kisi behaviour:
 *
 *   - usesKisiKisi=false: question accordions show no kisi-kisi
 *     subsection. Pure content + options + stimulus.
 *
 *   - usesKisiKisi=true (with or without template): the question
 *     accordion adds a "Kisi-Kisi" subsection with KD / materi /
 *     indikator / cognitive level / difficulty (or AKM dimensions
 *     when blueprint_type is AKM). The backend auto-mints a slot
 *     per question on save, so the canvas no longer forces a
 *     "load template first" step.
 *
 *   - When a template is loaded the slot summary is rendered above
 *     each question and metadata fields are read-only.
 *
 * Drag-and-drop: per-parent reorder via @dnd-kit + cross-parent moves
 * via droppable section / group banners. Slot binding is preserved.
 */

import { useEffect, useMemo, useState } from "react";
import {
  DndContext,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  closestCorners,
  pointerWithin,
  useDroppable,
  DragOverlay,
  type CollisionDetection,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Loader2, Plus } from "lucide-react";
import {
  listQuestions,
  listExamSections,
  listExamGroups,
  getExamBlueprint,
  getSlotsWithQuestions,
  createExamSection,
  createQuestionGroup,
  deleteQuestion,
  moveQuestion,
  updateQuestionGroup,
  type Exam,
  type ExamSection,
  type ExamQuestionGroup,
  type ExamBlueprint,
  type Question,
  type SlotsWithQuestionsResponse,
} from "@/lib/modules-api";
import { useToast } from "@/components/ui/toast";
import { Skeleton } from "@/components/ui/skeleton";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { CoverageBadge } from "@/components/blueprint/coverage-badge";
import { QuestionAccordion } from "@/components/exams/question-accordion";
import { GroupCard } from "@/components/exams/group-card";
import { SectionCard } from "@/components/exams/section-card";
import { cn } from "@/lib/cn";

export interface ExamCanvasProps {
  exam: Exam;
  onExamChange?: () => void;
  onBlueprintTypeChange?: (bpType: string | null) => void;
  /** Reverse-flow trigger \u2014 wired to AI chat by the parent page. */
  onGenerateFromQuestions?: () => void;
  /** Open the LoadKisiKisiSheet modal. */
  onRequestLoadKisiKisi?: () => void;
}

export function ExamCanvas({
  exam,
  onBlueprintTypeChange,
}: ExamCanvasProps) {
  const { toast } = useToast();
  const canEdit = exam.status === "draft";

  const [loading, setLoading] = useState(true);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [sections, setSections] = useState<ExamSection[]>([]);
  const [groups, setGroups] = useState<ExamQuestionGroup[]>([]);
  const [blueprint, setBlueprint] = useState<ExamBlueprint | null>(null);
  const [slotsResp, setSlotsResp] = useState<SlotsWithQuestionsResponse | null>(
    null,
  );

  // Open accordion + draft slot. Only one accordion open at a time.
  const [openId, setOpenId] = useState<string | null>(null);
  type Draft = {
    id: string;
    sectionId?: string | null;
    groupId?: string | null;
  };
  const [draft, setDraft] = useState<Draft | null>(null);

  // Confirm-delete state.
  const [deleteTarget, setDeleteTarget] = useState<Question | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Active drag id for the DragOverlay portal. Stored as the prefixed id
  // ('q:..' or 'g:..') so we can route to the right preview component.
  const [activeDragId, setActiveDragId] = useState<string | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  async function reload() {
    setLoading(true);
    const [qRes, sRes, gRes] = await Promise.all([
      listQuestions(exam.id),
      listExamSections(exam.id),
      listExamGroups(exam.id),
    ]);
    if (qRes.data) setQuestions(qRes.data.data);
    if (sRes.data) setSections(sRes.data.data);
    if (gRes.data) setGroups(gRes.data.data);

    if (exam.usesKisiKisi) {
      const [bpRes, slotsR] = await Promise.all([
        getExamBlueprint(exam.id),
        getSlotsWithQuestions(exam.id),
      ]);
      setBlueprint(bpRes.data?.blueprint ?? null);
      const sd = slotsR.data ?? null;
      setSlotsResp(sd);
      onBlueprintTypeChange?.(sd?.blueprintType ?? null);
    } else {
      setBlueprint(null);
      setSlotsResp(null);
      onBlueprintTypeChange?.(null);
    }
    setLoading(false);
  }

  useEffect(() => {
    reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exam.id, exam.usesKisiKisi, exam.status]);

  useEffect(() => {
    function h() {
      reload();
    }
    window.addEventListener("morfoschools:data-changed", h);
    return () =>
      window.removeEventListener("morfoschools:data-changed", h);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exam.id]);

  // --- Mutations + helpers ---

  async function handleAddSection() {
    const nextIndex = sections.length + 1;
    const res = await createExamSection(exam.id, {
      title: `Section ${nextIndex}`,
    });
    if (res.error) {
      toast({
        tone: "error",
        title: "Gagal menambah section",
        description: res.error.message,
      });
      return;
    }
    toast({ tone: "success", title: "Section ditambahkan" });
    reload();
  }

  async function handleAddGroup(sectionId?: string) {
    const res = await createQuestionGroup(exam.id, { sectionId });
    if (res.error || !res.data) {
      toast({
        tone: "error",
        title: "Gagal menambah group",
        description: res.error?.message ?? "Tidak diketahui",
      });
      return;
    }
    toast({ tone: "success", title: "Group dibuat" });
    reload();
  }

  function handleAddQuestion(opts: {
    sectionId?: string | null;
    groupId?: string | null;
  }) {
    const id = `draft:${Date.now()}`;
    setDraft({ id, ...opts });
    setOpenId(id);
  }

  async function handleDelete(q: Question) {
    setDeleting(true);
    const res = await deleteQuestion(q.id);
    setDeleting(false);
    if (res.error) {
      toast({
        tone: "error",
        title: "Gagal menghapus",
        description: res.error.message,
      });
      return;
    }
    toast({ tone: "success", title: "Soal dihapus" });
    setDeleteTarget(null);
    if (openId === q.id) setOpenId(null);
    reload();
  }

  // --- DnD: unified block reorder + cross-parent moves ---
  //
  // A "block" is either a group or a standalone question. Within a
  // section, blocks share a single ordering axis: groups carry it in
  // exam_question_groups.display_order, standalone questions carry it
  // in exam_questions.sort_order. The frontend recomputes positions
  // 0..N-1 across the unified block list on every drop and pushes one
  // PATCH per block. Order is normalized on every drop so collisions
  // between the two columns are eliminated.
  function handleDragStart(evt: DragStartEvent) {
    setActiveDragId(String(evt.active.id));
  }

  async function handleDragEnd(evt: DragEndEvent) {
    setActiveDragId(null);
    const { active, over } = evt;
    if (!over) return;
    const activeID = String(active.id);
    const overID = String(over.id);
    if (activeID === overID) return;

    const activeKind = activeID.split(":")[0];
    const activeRefId = activeID.split(":")[1];
    const overKind = overID.split(":")[0];
    const overRefId = overID.split(":")[1];

    // Question dropped onto a group's body — join that group.
    if (activeKind === "q" && overKind === "drop-group") {
      const q = questions.find((x) => x.id === activeRefId);
      if (!q || q.groupId === overRefId) return;
      const res = await moveQuestion(exam.id, {
        questionId: activeRefId,
        groupId: overRefId,
      });
      if (res.error) {
        toast({
          tone: "error",
          title: "Move failed",
          description: res.error.message,
        });
        return;
      }
      reload();
      return;
    }

    // Question dropped onto a section banner — leave any group, land in
    // that section as a standalone question.
    if (activeKind === "q" && overKind === "drop-section") {
      const q = questions.find((x) => x.id === activeRefId);
      if (!q || (q.sectionId === overRefId && !q.groupId)) return;
      const res = await moveQuestion(exam.id, {
        questionId: activeRefId,
        sectionId: overRefId,
        groupId: "",
      });
      if (res.error) {
        toast({
          tone: "error",
          title: "Move failed",
          description: res.error.message,
        });
        return;
      }
      reload();
      return;
    }

    // Question-to-question intents:
    //   1. Same group: reorder inside that group.
    //   2. Different groups: move active into over's group.
    //   3. Active in group, over standalone (root): leave group, land
    //      in over's section as a standalone, sort_order = over's.
    //   4. Active standalone (root), over in group: join over's group.
    if (activeKind === "q" && overKind === "q") {
      const a = questions.find((x) => x.id === activeRefId);
      const o = questions.find((x) => x.id === overRefId);
      if (!a || !o) return;

      // Case 1: same group reorder.
      if (a.groupId && a.groupId === o.groupId) {
        const siblings = questions
          .filter((x) => x.groupId === a.groupId)
          .sort((x, y) => x.sortOrder - y.sortOrder);
        const oldIdx = siblings.findIndex((x) => x.id === a.id);
        const newIdx = siblings.findIndex((x) => x.id === o.id);
        if (oldIdx < 0 || newIdx < 0 || oldIdx === newIdx) return;
        const reordered = [...siblings];
        reordered.splice(oldIdx, 1);
        reordered.splice(newIdx, 0, a);
        const calls = reordered.map((q, i) =>
          moveQuestion(exam.id, { questionId: q.id, sortOrder: i }),
        );
        const results = await Promise.all(calls);
        const failed = results.find((r) => r.error);
        if (failed?.error) {
          toast({
            tone: "error",
            title: "Reorder failed",
            description: failed.error.message,
          });
        }
        reload();
        return;
      }

      // Case 2 + 3 + 4: cross-group / leave / join. Resolve to a single
      // moveQuestion call setting both groupId and sectionId.
      if (a.groupId !== o.groupId) {
        const targetGroupId = o.groupId ?? "";
        const targetSectionId = o.sectionId ?? "";
        const res = await moveQuestion(exam.id, {
          questionId: a.id,
          groupId: targetGroupId,
          sectionId: targetSectionId,
        });
        if (res.error) {
          toast({
            tone: "error",
            title: "Move failed",
            description: res.error.message,
          });
          return;
        }
        reload();
        return;
      }
    }

    // Section-level block reorder: blocks (groups + standalone
    // questions) interleaved within one section. Active and over are
    // both block-level (g:X, q:Y where Y has no groupId) and share a
    // section. We recompute positions across the merged list and PATCH
    // each affected row.
    const activeBlockSection = blockSectionId(activeID);
    const overBlockSection = blockSectionId(overID);
    if (
      activeBlockSection &&
      overBlockSection &&
      activeBlockSection === overBlockSection
    ) {
      const blocks = buildSectionBlocks(
        activeBlockSection,
        groups,
        questions,
      );
      const oldIdx = blocks.findIndex((b) => blockKey(b) === activeID);
      const newIdx = blocks.findIndex((b) => blockKey(b) === overID);
      if (oldIdx < 0 || newIdx < 0 || oldIdx === newIdx) return;
      const reordered = [...blocks];
      const [moved] = reordered.splice(oldIdx, 1);
      reordered.splice(newIdx, 0, moved);
      const calls = reordered.map((b, i) => {
        if (b.kind === "group") {
          return updateQuestionGroup(b.group.id, { sortOrder: i });
        }
        return moveQuestion(exam.id, {
          questionId: b.question.id,
          sortOrder: i,
        });
      });
      const results = await Promise.all(calls);
      const failed = results.find((r) => r.error);
      if (failed?.error) {
        toast({
          tone: "error",
          title: "Reorder failed",
          description: failed.error.message,
        });
      }
      reload();
      return;
    }

    // Helper: returns the sectionId for any active/over id at block
    // level (either g:groupId or q:standaloneQuestionId). Returns null
    // if the id refers to a question inside a group (not a block).
    function blockSectionId(rawId: string): string | null {
      const [kind, ref] = rawId.split(":");
      if (kind === "g") {
        const g = groups.find((x) => x.id === ref);
        return g?.sectionId ?? null;
      }
      if (kind === "q") {
        const q = questions.find((x) => x.id === ref);
        if (!q) return null;
        return q.groupId ? null : q.sectionId ?? null;
      }
      return null;
    }
  }

  // --- Build the tree ---
  const tree = useMemo(() => {
    const bySection: Record<string, Question[]> = {};
    const byGroup: Record<string, Question[]> = {};
    for (const q of questions) {
      if (q.groupId) {
        (byGroup[q.groupId] ??= []).push(q);
      } else if (q.sectionId) {
        (bySection[q.sectionId] ??= []).push(q);
      }
    }
    for (const k of Object.keys(bySection))
      bySection[k].sort((a, b) => a.sortOrder - b.sortOrder);
    for (const k of Object.keys(byGroup))
      byGroup[k].sort((a, b) => a.sortOrder - b.sortOrder);
    return { bySection, byGroup };
  }, [questions]);

  // Continuous question number across the whole canvas. The map walks
  // sections in order, blocks within each section in display order,
  // and questions inside each group in their group-local sort order.
  // Standalone-in-section questions count too. Result: { questionId: N }
  // where N starts at 1 and is global to the exam.
  const questionNumber = useMemo(() => {
    const map: Record<string, number> = {};
    let n = 1;
    for (const section of sections) {
      const blocks = buildSectionBlocks(section.id, groups, questions);
      for (const b of blocks) {
        if (b.kind === "group") {
          const inGroup = (tree.byGroup[b.group.id] ?? []).slice().sort(
            (a, c) => a.sortOrder - c.sortOrder,
          );
          for (const q of inGroup) {
            map[q.id] = n++;
          }
        } else {
          map[b.question.id] = n++;
        }
      }
    }
    return map;
  }, [sections, groups, questions, tree.byGroup]);

  // Custom collision detection. Reasons (from user feedback):
  //
  // 1. closestCenter picks the nearest item by centerpoint. A group
  //    containing 4 inner questions is tall — its center sits well
  //    below the visual top edge. Dragging another item to position
  //    above the group required the cursor to be far above the group's
  //    visible top, which feels unresponsive.
  //
  // 2. Inner-group questions register as `q:` sortables in their own
  //    nested SortableContext. When the user drags a block-level item
  //    (a group `g:` or a root question `q:`) past a tall group, the
  //    cursor often hovers over one of those inner questions and dnd-
  //    kit picks it as the "over" target. handleDragEnd then has no
  //    valid path for that combination and silently no-ops.
  //
  // The custom detector below:
  //   - prefers pointerWithin (cursor inside a rectangle) for clear
  //     drops, falling back to closestCorners for mixed-height tall
  //     blocks like groups
  //   - filters inner-group questions out of the collision set when
  //     the active item is block-level, so a group reorder cannot
  //     accidentally target an inner question of an unrelated group
  //   - filters block-level peers out when the active item is itself
  //     inside a group, so an inside-group reorder doesn't slip into
  //     the section-level block list
  const collisionDetection = useMemo<CollisionDetection>(() => {
    // Custom two-stage collision detection.
    //
    // Stage 1 — STRONG intent: cursor literally inside a drop-group or
    // drop-section wrapper that represents a NON-NO-OP target for the
    // active item. Returns those wrappers so handleDragEnd routes via
    // the join/leave/cross-section paths.
    //
    // Stage 2 — WEAK intent: cursor between siblings or no useful drop
    // wrapper present. Returns the closest sortable peer (group or
    // question) for handleDragEnd's reorder branches.
    //
    // The bug-prone bit was Stage 2: the previous version stripped
    // drop wrappers from the pointerWithin output, then checked
    // length === 0 BEFORE fallback to closestCorners. That meant a
    // drag in the section gutter (where pointerWithin = [drop-section:A]
    // only) ended up with collisions = [] and silently no-opped.
    return (args) => {
      const activeId = String(args.active.id);
      const [activeKind, activeRef] = activeId.split(":");

      // Resolve the active item's parents from canonical state.
      let activeSection: string | null = null;
      let activeGroup: string | null = null;
      if (activeKind === "q") {
        const q = questions.find((x) => x.id === activeRef);
        activeSection = q?.sectionId ?? null;
        activeGroup = q?.groupId ?? null;
      } else if (activeKind === "g") {
        const g = groups.find((x) => x.id === activeRef);
        activeSection = g?.sectionId ?? null;
      }

      const within = pointerWithin(args);

      // Is the cursor still inside its own group's body?
      // (For inner-group active: distinguishes "reorder inside" from
      //  "leave the group via section gutter".)
      const insideOwnGroupBody = within.some((c) => {
        const [k, ref] = String(c.id).split(":");
        return k === "drop-group" && ref === activeGroup;
      });

      // Stage 1 — strong intents.
      const dropTargets = within.filter((c) => {
        const [k, ref] = String(c.id).split(":");

        if (k === "drop-section") {
          if (ref !== activeSection) return true; // cross-section move
          // Same section. No-op when active is already at section root.
          if (activeKind === "g") return false;
          if (activeKind === "q" && !activeGroup) return false;
          // Inner-group q in same section:
          //   inside own group body → no-op (let Stage 2 do the
          //                                  in-group reorder)
          //   outside own group body → valid leave-group intent
          if (insideOwnGroupBody) return false;
          return true;
        }

        if (k === "drop-group") {
          if (activeKind === "g") return false; // groups can't nest
          if (ref === activeGroup) return false; // already inside
          return true;
        }
        return false;
      });

      if (dropTargets.length > 0) {
        // Prefer drop-group over drop-section when both qualify (more
        // specific intent wins).
        dropTargets.sort((a, b) => {
          const aSpec = String(a.id).startsWith("drop-group") ? 0 : 1;
          const bSpec = String(b.id).startsWith("drop-group") ? 0 : 1;
          return aSpec - bSpec;
        });
        return dropTargets;
      }

      // Stage 2 — weak intent: nearest sibling for reorder.
      // Strip drop wrappers FIRST, then fall back to closestCorners
      // when nothing useful is in within. (This is the fix.)
      const stripWrappers = (list: Array<{ id: string | number }>) =>
        list.filter((c) => {
          const k = String(c.id).split(":")[0];
          return k !== "drop-group" && k !== "drop-section";
        });

      let collisions = stripWrappers(within);
      if (collisions.length === 0) {
        collisions = stripWrappers(closestCorners(args));
      }
      return collisions;
    };
  }, [questions, groups]);

  // --- Render ---

  if (loading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
      </div>
    );
  }

  const isAkm =
    blueprint?.blueprintType === "akm_literasi" ||
    blueprint?.blueprintType === "akm_numerasi";
  const sourceTemplateId = slotsResp?.sourceTemplateId ?? null;


  return (
    <DndContext
      sensors={sensors}
      collisionDetection={collisionDetection}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragCancel={() => setActiveDragId(null)}
    >
      <div className="space-y-3">
        {/* Blueprint header strip (when KK on + template loaded) */}
        {exam.usesKisiKisi && blueprint && (
          <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-[var(--border)] bg-[var(--card)] px-3 py-2.5">
            <div className="min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <h3 className="text-[13px] font-semibold text-[var(--foreground)]">
                  {blueprint.title}
                </h3>
                <span className="rounded-md bg-[var(--accent)] px-2 py-0.5 text-[10px] font-medium text-[var(--muted-foreground)]">
                  {blueprint.curriculumCode.toUpperCase()} \u00b7{" "}
                  {blueprint.blueprintType.replace("akm_", "AKM ")}
                </span>
              </div>
            </div>
            {slotsResp && (
              <CoverageBadge
                filled={slotsResp.coverage.filled}
                total={slotsResp.coverage.total}
                strict={blueprint.strictCoverage}
              />
            )}
          </div>
        )}

        {/* Sections */}
        {sections.map((section) => {
          const blocks = buildSectionBlocks(section.id, groups, questions);
          const blockIds = blocks.map(blockKey);
          return (
            <SectionDropTarget key={section.id} sectionId={section.id}>
              <SectionCard
                section={section}
                questionCount={
                  blocks.reduce(
                    (n, b) =>
                      n +
                      (b.kind === "group"
                        ? (tree.byGroup[b.group.id] ?? []).length
                        : 1),
                    0,
                  )
                }
                canEdit={canEdit}
                isOnlySection={sections.length === 1}
                onAddQuestion={() =>
                  handleAddQuestion({ sectionId: section.id })
                }
                onAddGroup={() => handleAddGroup(section.id)}
                onChange={reload}
              >
                <SortableContext
                  items={blockIds}
                  strategy={verticalListSortingStrategy}
                >
                  {blocks.map((b) =>
                    b.kind === "group" ? (
                      <GroupBlock
                        key={b.group.id}
                        group={b.group}
                        questions={tree.byGroup[b.group.id] ?? []}
                        questionNumber={questionNumber}
                        canEdit={canEdit}
                        openId={openId}
                        draft={draft}
                        usesKisiKisi={exam.usesKisiKisi}
                        isAkm={isAkm}
                        sourceTemplateId={sourceTemplateId}
                        onToggle={(id) =>
                          setOpenId((p) => (p === id ? null : id))
                        }
                        onAddDraft={() =>
                          handleAddQuestion({
                            sectionId: section.id,
                            groupId: b.group.id,
                          })
                        }
                        onCancelDraft={() => {
                          setDraft(null);
                          setOpenId(null);
                        }}
                        onSavedDraft={() => {
                          setDraft(null);
                          setOpenId(null);
                          reload();
                        }}
                        onSaved={() => {
                          setOpenId(null);
                          reload();
                        }}
                        onDeleteRequest={(q) => setDeleteTarget(q)}
                        examId={exam.id}
                        onChange={reload}
                      />
                    ) : (
                      <SortableQuestionRow
                        key={b.question.id}
                        question={b.question}
                        examId={exam.id}
                        canEdit={canEdit}
                        isOpen={openId === b.question.id}
                        usesKisiKisi={exam.usesKisiKisi}
                        isAkm={isAkm}
                        sourceTemplateId={sourceTemplateId}
                        onToggle={() =>
                          setOpenId((p) =>
                            p === b.question.id ? null : b.question.id,
                          )
                        }
                        onSaved={() => {
                          setOpenId(null);
                          reload();
                        }}
                        onDelete={() => setDeleteTarget(b.question)}
                        index={questionNumber[b.question.id] ?? 0}
                      />
                    ),
                  )}
                  {/* Draft accordion scoped to this section (not in a group) */}
                  {draft &&
                    draft.sectionId === section.id &&
                    !draft.groupId && (
                      <QuestionAccordion
                        question={null}
                        isDraft
                        onCancelDraft={() => {
                          setDraft(null);
                          setOpenId(null);
                        }}
                        examId={exam.id}
                        canEdit={canEdit}
                        isOpen
                        onToggle={() => {
                          setDraft(null);
                          setOpenId(null);
                        }}
                        onSaved={() => {
                          setDraft(null);
                          setOpenId(null);
                          reload();
                        }}
                        index={Object.keys(questionNumber).length + 1}
                        defaultSectionId={section.id}
                        usesKisiKisi={exam.usesKisiKisi}
                        isAkm={isAkm}
                        slotLockedFromTemplate={false}
                      />
                    )}
                </SortableContext>
              </SectionCard>
            </SectionDropTarget>
          );
        })}

        {/* Add-section CTA (full-width dashed) */}
        {canEdit && (
          <button
            type="button"
            onClick={handleAddSection}
            className="flex w-full items-center justify-center gap-1.5 rounded-xl border-2 border-dashed border-[var(--border-strong)] bg-transparent py-3 text-[12px] font-medium text-[var(--muted-foreground)] transition-colors hover:border-[var(--brand)] hover:bg-[var(--brand-soft)]/30 hover:text-[var(--brand)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand)]/40"
          >
            <Plus size={13} /> Tambah Section
          </button>
        )}
      </div>

      <DragOverlay dropAnimation={{ duration: 160, easing: "cubic-bezier(0.18, 0.67, 0.6, 1.22)" }}>
        {activeDragId ? (
          <DragPreview activeId={activeDragId} questions={questions} groups={groups} />
        ) : null}
      </DragOverlay>

      <ConfirmDialog
        open={!!deleteTarget}
        title="Hapus soal?"
        description="Soal akan dihapus permanen. Tidak bisa dibatalkan."
        confirmLabel="Hapus"
        destructive
        loading={deleting}
        onConfirm={() => deleteTarget && handleDelete(deleteTarget)}
        onCancel={() => setDeleteTarget(null)}
      />
    </DndContext>
  );
}

// \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500
// Sortable + droppable subcomponents
// \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500

function SortableQuestionRow(props: {
  question: Question;
  examId: string;
  canEdit: boolean;
  isOpen: boolean;
  usesKisiKisi: boolean;
  isAkm: boolean;
  sourceTemplateId: string | null;
  onToggle: () => void;
  onSaved: () => void;
  onDelete: () => void;
  index: number;
  isInsideGroup?: boolean;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: `q:${props.question.id}` });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0 : 1,
  };
  const slotLocked =
    !!props.sourceTemplateId &&
    !!(props.question.slot && props.question.slot.fromTemplate !== false);

  return (
    <div ref={setNodeRef} style={style}>
      <QuestionAccordion
        question={props.question}
        examId={props.examId}
        canEdit={props.canEdit}
        isOpen={props.isOpen}
        onToggle={props.onToggle}
        onSaved={props.onSaved}
        onDelete={props.onDelete}
        dragHandleProps={
          props.canEdit ? { ...attributes, ...listeners } : undefined
        }
        index={props.index}
        isInsideGroup={props.isInsideGroup}
        usesKisiKisi={props.usesKisiKisi}
        isAkm={props.isAkm}
        slotLockedFromTemplate={slotLocked}
      />
    </div>
  );
}

function GroupBlock(props: {
  group: ExamQuestionGroup;
  questions: Question[];
  questionNumber: Record<string, number>;
  canEdit: boolean;
  openId: string | null;
  draft: { id: string; groupId?: string | null; sectionId?: string | null } | null;
  usesKisiKisi: boolean;
  isAkm: boolean;
  sourceTemplateId: string | null;
  onToggle: (id: string) => void;
  onAddDraft: () => void;
  onCancelDraft: () => void;
  onSavedDraft: () => void;
  onSaved: () => void;
  onDeleteRequest: (q: Question) => void;
  examId: string;
  onChange: () => void;
}) {
  const {
    group,
    questions,
    questionNumber,
    canEdit,
    openId,
    draft,
    usesKisiKisi,
    isAkm,
    sourceTemplateId,
    onToggle,
    onAddDraft,
    onCancelDraft,
    onSavedDraft,
    onSaved,
    onDeleteRequest,
    examId,
    onChange,
  } = props;

  const sortable = useSortable({ id: `g:${group.id}` });
  const style = {
    transform: CSS.Transform.toString(sortable.transform),
    transition: sortable.transition,
    opacity: sortable.isDragging ? 0 : 1,
  };
  const drop = useDroppable({ id: `drop-group:${group.id}` });

  return (
    <div ref={sortable.setNodeRef} style={style}>
      <div
        ref={drop.setNodeRef}
        className={cn(
          "transition-shadow rounded-xl",
          drop.isOver && "ring-2 ring-[var(--brand)]/40",
        )}
      >
        <GroupCard
          group={group}
          questionCount={questions.length}
          canEdit={canEdit}
          examId={examId}
          onAddQuestion={onAddDraft}
          onChange={onChange}
          dragHandleProps={
            canEdit
              ? { ...sortable.attributes, ...sortable.listeners }
              : undefined
          }
        >
          <SortableContext
            items={questions.map((q) => `q:${q.id}`)}
            strategy={verticalListSortingStrategy}
          >
            {questions.map((q) => (
              <SortableQuestionRow
                key={q.id}
                question={q}
                examId={examId}
                canEdit={canEdit}
                isOpen={openId === q.id}
                usesKisiKisi={usesKisiKisi}
                isAkm={isAkm}
                sourceTemplateId={sourceTemplateId}
                onToggle={() => onToggle(q.id)}
                onSaved={onSaved}
                onDelete={() => onDeleteRequest(q)}
                index={questionNumber[q.id] ?? 0}
                isInsideGroup
              />
            ))}
            {draft && draft.groupId === group.id && (
              <QuestionAccordion
                question={null}
                isDraft
                onCancelDraft={onCancelDraft}
                examId={examId}
                canEdit={canEdit}
                isOpen
                onToggle={onCancelDraft}
                onSaved={onSavedDraft}
                index={questions.length + 1}
                isInsideGroup
                defaultGroupId={group.id}
                defaultSectionId={draft.sectionId ?? undefined}
                usesKisiKisi={usesKisiKisi}
                isAkm={isAkm}
                slotLockedFromTemplate={false}
              />
            )}
          </SortableContext>
        </GroupCard>
      </div>
    </div>
  );
}

function SectionDropTarget({
  sectionId,
  children,
}: {
  sectionId: string;
  children: React.ReactNode;
}) {
  const drop = useDroppable({ id: `drop-section:${sectionId}` });
  return (
    <div
      ref={drop.setNodeRef}
      className={cn(
        "rounded-xl transition-shadow",
        drop.isOver && "ring-2 ring-[var(--brand)]/40",
      )}
    >
      {children}
    </div>
  );
}

// Loader2 retained for compatibility with future inline loading states.
// ────────────────────────────────────────────────────────────────────
// Section blocks: groups + standalone questions interleaved.
//
// Within a section, groups carry display_order and standalone questions
// carry sort_order. To support drag-and-drop reordering across the two,
// we merge them into a single list ordered by their respective field,
// breaking ties by id for determinism. Frontend recomputes 0..N-1
// positions on every drop and the backend stores the resolved values.
// ────────────────────────────────────────────────────────────────────

type Block =
  | { kind: "group"; group: ExamQuestionGroup; order: number }
  | { kind: "question"; question: Question; order: number };

function blockKey(b: Block): string {
  return b.kind === "group" ? `g:${b.group.id}` : `q:${b.question.id}`;
}

function buildSectionBlocks(
  sectionId: string,
  groups: ExamQuestionGroup[],
  questions: Question[],
): Block[] {
  const sectionGroups = groups
    .filter((g) => g.sectionId === sectionId)
    .map<Block>((g) => ({ kind: "group", group: g, order: g.displayOrder }));
  const standalone = questions
    .filter((q) => q.sectionId === sectionId && !q.groupId)
    .map<Block>((q) => ({ kind: "question", question: q, order: q.sortOrder }));
  return [...sectionGroups, ...standalone].sort((a, b) => {
    if (a.order !== b.order) return a.order - b.order;
    if (a.kind !== b.kind) return a.kind === "group" ? -1 : 1;
    const aId = a.kind === "group" ? a.group.id : a.question.id;
    const bId = b.kind === "group" ? b.group.id : b.question.id;
    return aId.localeCompare(bId);
  });
}

void Loader2;


function DragPreview({
  activeId,
  questions,
  groups,
}: {
  activeId: string;
  questions: Question[];
  groups: ExamQuestionGroup[];
}) {
  const [kind, ref] = activeId.split(":");
  if (kind === "q") {
    const q = questions.find((x) => x.id === ref);
    if (!q) return null;
    const preview = stripPreview(q.content);
    return (
      <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] px-3 py-2.5 text-[12px] text-[var(--foreground)] shadow-[0_12px_32px_rgba(0,0,0,0.18)] max-w-[640px]">
        <div className="flex items-center gap-2">
          <span className="rounded-md bg-[var(--muted)] px-1.5 py-0.5 text-[10px] font-semibold text-[var(--muted-foreground)]">
            {q.questionType.replace("_", " ")}
          </span>
          <span className="truncate flex-1">{preview || "(soal kosong)"}</span>
        </div>
      </div>
    );
  }
  if (kind === "g") {
    const g = groups.find((x) => x.id === ref);
    if (!g) return null;
    const title = g.stimulusTitleSnapshot || "Group";
    return (
      <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] px-3 py-2.5 text-[12px] text-[var(--foreground)] shadow-[0_12px_32px_rgba(0,0,0,0.18)] max-w-[640px]">
        <div className="flex items-center gap-2">
          <span className="rounded-md bg-[var(--brand-soft)] px-1.5 py-0.5 text-[10px] font-semibold text-[var(--brand)]">
            Group
          </span>
          <span className="truncate flex-1">{title}</span>
          <span className="rounded-md bg-[var(--muted)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--muted-foreground)]">
            {g.questionCount ?? 0} soal
          </span>
        </div>
      </div>
    );
  }
  return null;
}

function stripPreview(html: string | null | undefined): string {
  if (!html) return "";
  return html.replace(/<[^>]+>/g, "").replace(/\s+/g, " ").trim().slice(0, 80);
}

