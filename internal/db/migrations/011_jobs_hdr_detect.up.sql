-- Migration 011: extend jobs job_type to include hdr_detect
--
-- The hdr_detect job type was introduced with migration 010 (source HDR
-- columns) but the jobs table CHECK constraint was not updated at that time.
-- This migration closes that gap.

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_job_type_check;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_job_type_check
    CHECK (job_type IN ('encode', 'analysis', 'audio', 'hdr_detect'));
