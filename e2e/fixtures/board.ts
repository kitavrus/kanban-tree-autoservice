import { test as base } from './test';
import type { Column } from './board-types';

// ADR-101: подзадачи (услуги в заказе) и принятое оборудование заказа заводятся через
// боевое API карточки заказа фазы 6 (internal/features/board/card_*), не прямой SQL-
// вставкой (pg удалён из зависимостей e2e) — те же гарантии, что и у настоящего клиента
// (AdminApi.createSubtask/updateSubtask/createSubtaskProduct/deleteSubtaskProduct/
// addTaskEquipment, e2e/fixtures/auth.ts).

export type SubtaskStatus = 'new' | 'in_progress' | 'done';

export type NewSubtask = {
  taskId: number;
  serviceId: string;
  status?: SubtaskStatus;
  dueDate?: string | null;
  products?: Array<{ productId: string; qty: number }>;
};

type BoardFixtures = {
  /** Создаёт услугу заказа через POST /tasks/:id/subtasks (+ товары/статус/срок при
   *  необходимости) — возвращает id услуги. */
  insertSubtask: (s: NewSubtask) => Promise<string>;
  /** Меняет статус услуги через PUT /subtasks/:id (полная замена строки — LWW; остальные
   *  поля сохраняются как в текущей карточке). */
  setSubtaskStatus: (subtaskId: string, status: SubtaskStatus) => Promise<void>;
  /** Отмечает оборудование клиента как принятое в заказе (POST /tasks/:id/equipment). */
  insertTaskEquipment: (taskId: number, equipmentId: string) => Promise<void>;
  /** Внутренний кэш subtaskId → taskId (для setSubtaskStatus — у PUT /subtasks/:id нет
   *  обратной ссылки на заказ в URL). */
  _subtaskTaskMap: Map<string, number>;
};

export const test = base.extend<BoardFixtures>({
  _subtaskTaskMap: async ({}, use) => {
    await use(new Map<string, number>());
  },

  insertSubtask: async ({ adminApi, _subtaskTaskMap }, use) => {
    await use(async (s) => {
      const created = await adminApi.createSubtask(s.taskId, s.serviceId);
      // POST добавляет ОДНУ новую услугу в конец — она с максимальной position.
      const sub = created.card.subtasks.reduce((a, b) => (a.position > b.position ? a : b));
      _subtaskTaskMap.set(sub.id, s.taskId);

      if (s.products) {
        // комплект, скопированный из каталога (AC-008), заменяется явным набором теста.
        for (const p of sub.products) {
          await adminApi.deleteSubtaskProduct(p.id);
        }
        for (const p of s.products) {
          await adminApi.createSubtaskProduct(sub.id, { product_id: p.productId, qty: p.qty });
        }
      }
      if (s.status !== undefined || s.dueDate !== undefined) {
        await adminApi.updateSubtask(sub.id, { status: s.status ?? 'new', due_date: s.dueDate ?? null });
      }
      return sub.id;
    });
  },

  setSubtaskStatus: async ({ adminApi, _subtaskTaskMap }, use) => {
    await use(async (subtaskId, status) => {
      const taskId = _subtaskTaskMap.get(subtaskId);
      if (taskId === undefined) {
        throw new Error(`setSubtaskStatus: подзадача ${subtaskId} создана не через insertSubtask этого теста`);
      }
      const card = await adminApi.getCard(taskId);
      const sub = card.subtasks.find((x) => x.id === subtaskId);
      if (!sub) {
        throw new Error(`setSubtaskStatus: подзадача ${subtaskId} не найдена в карточке заказа ${taskId}`);
      }
      await adminApi.updateSubtask(subtaskId, {
        status,
        assignee_id: sub.assignee_id,
        due_date: sub.due_date,
        price: sub.price,
        equipment_id: sub.equipment_id,
        // is_executor_paid / executor_payout не шлём: нет поля → сервер их не меняет (D65).
      });
    });
  },

  insertTaskEquipment: async ({ adminApi }, use) => {
    await use(async (taskId, equipmentId) => {
      await adminApi.addTaskEquipment(taskId, equipmentId);
    });
  },
});

/** Колонка сида D32 по названию (AC-008) — колонки создаются миграцией, id непредсказуем. */
export function columnByName(columns: Column[], name: string): Column {
  const c = columns.find((x) => x.name === name);
  if (!c) throw new Error(`колонка «${name}» не найдена среди: ${columns.map((x) => x.name).join(', ')}`);
  return c;
}

export { expect } from './test';
