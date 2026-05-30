-- Exam delivery gates moved to Program Items.
-- Exams are reusable authoring objects; class/user availability must be
-- evaluated in the Program Item delivery context, not on the exam itself.
DROP TABLE IF EXISTS exam_gate_windows;
