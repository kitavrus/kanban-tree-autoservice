-- Выплаты исполнителям (D62–D69): статус «выплачено ли исполнителю за услугу заказа».
-- Флаг и дата — как is_paid/paid_at заказа; executor_payout — сумма выплаты исполнителю,
-- отдельная от price (цена для клиента). Связь «отметка только при назначенном исполнителе»
-- держит сервис, не CHECK: subtasks.assignee_id — ON DELETE SET NULL (удаление сотрудника
-- не должно упираться в выплаченные услуги).
ALTER TABLE subtasks
    ADD COLUMN is_executor_paid boolean NOT NULL DEFAULT false,
    ADD COLUMN executor_paid_at timestamptz NULL,
    ADD COLUMN executor_payout  numeric(12,2) NULL
        CONSTRAINT subtasks_executor_payout_check CHECK (executor_payout IS NULL OR executor_payout >= 0);

-- Отчёт «Выплаты» и фильтр доски выбирают невыплаченные услуги с исполнителем.
CREATE INDEX subtasks_executor_unpaid_idx ON subtasks (assignee_id)
    WHERE assignee_id IS NOT NULL AND NOT is_executor_paid;
