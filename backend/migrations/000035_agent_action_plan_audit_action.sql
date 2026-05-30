-- Allow batch action_type=audit and broader target_type values for kisi-kisi execution plans.
ALTER TABLE agent_action_plan_batches
    DROP CONSTRAINT IF EXISTS agent_action_plan_batches_action_type_check,
    ADD CONSTRAINT agent_action_plan_batches_action_type_check
        CHECK (action_type IN ('analyze', 'audit', 'create', 'update', 'repair', 'merge', 'link', 'generate', 'finalize'));

ALTER TABLE agent_action_plan_batches
    DROP CONSTRAINT IF EXISTS agent_action_plan_batches_target_type_check,
    ADD CONSTRAINT agent_action_plan_batches_target_type_check
        CHECK (target_type IN ('blueprint_slot', 'blueprint_slots', 'question', 'questions', 'exam', 'section', 'question_set', 'generic'));
