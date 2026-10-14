-- Откат выплат исполнителям: отметки, даты и суммы выплат теряются.
DROP INDEX IF EXISTS subtasks_executor_unpaid_idx;
ALTER TABLE subtasks
    DROP CONSTRAINT IF EXISTS subtasks_executor_payout_check,
    DROP COLUMN executor_payout,
    DROP COLUMN executor_paid_at,
    DROP COLUMN is_executor_paid;
