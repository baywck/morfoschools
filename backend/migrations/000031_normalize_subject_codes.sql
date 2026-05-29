-- Subject code is the canonical slug representation of subject name.
-- Historical rows may carry legacy/import slugs (for example Pendidikan Pancasila
-- stored as pendidikan-kewarganegaraan). Normalize them so CP lookup and UI
-- semantics stay aligned with the visible subject name.
UPDATE subjects
SET code = lower(regexp_replace(regexp_replace(trim(name), '[^[:alnum:]]+', '-', 'g'), '(^-+|-+$)', '', 'g')),
    updated_at = now()
WHERE code IS DISTINCT FROM lower(regexp_replace(regexp_replace(trim(name), '[^[:alnum:]]+', '-', 'g'), '(^-+|-+$)', '', 'g'));
