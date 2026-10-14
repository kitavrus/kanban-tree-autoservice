// Package board — доска заказов: колонки, заказы (BIGSERIAL, D37), подзадачи-услуги,
// движение склада по заказу (StockEngine.Reconcile, ADR-104). Многосущностный пакет,
// файлы разделены по сущности (эталон internal/features/catalog, D43).
package board

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Sentinel'ы, отдаваемые наружу API (design-errors.md §Sentinel Errors, коды 01-11).
var (
	ErrTaskNotFound         = errorspkg.New("TASK_NOT_FOUND", http.StatusNotFound, "Заказ не найден")
	ErrColumnNotFound       = errorspkg.New("COLUMN_NOT_FOUND", http.StatusNotFound, "Колонка не найдена")
	ErrSubtasksNotDone      = errorspkg.New("SUBTASKS_NOT_DONE", http.StatusConflict, "Есть незавершённые услуги — в финальную колонку нельзя")
	ErrColumnNotEmpty       = errorspkg.New("COLUMN_NOT_EMPTY", http.StatusConflict, "В колонке есть заказы")
	ErrLastNonFinalColumn   = errorspkg.New("LAST_NON_FINAL_COLUMN", http.StatusConflict, "Нужна хотя бы одна нефинальная колонка")
	ErrTaskVersionConflict  = errorspkg.New("TASK_VERSION_CONFLICT", http.StatusConflict, "Заказ уже изменён, обновите")
	ErrStockProductNotFound = errorspkg.New("PRODUCT_NOT_FOUND", http.StatusNotFound, "Товар не найден")
	// ErrStockDeletedWriteOffOnly — остаток товара, удалённого из каталога, можно только списать
	// (soft-delete-catalog DS-042, DS-021): увеличение либо смена знака — отказ.
	ErrStockDeletedWriteOffOnly = errorspkg.New("PRODUCT_DELETED_WRITE_OFF_ONLY", http.StatusConflict,
		"Товар удалён — его остаток можно только списать")
	// ErrStockWarehouseWriteOffOnly — остаток на мягко удалённом складе можно только списать
	// (soft-delete-warehouses-suppliers DS-048): увеличение либо смена знака — отказ.
	ErrStockWarehouseWriteOffOnly = errorspkg.New("WAREHOUSE_DELETED_WRITE_OFF_ONLY", http.StatusConflict,
		"Склад удалён — остаток на нём можно только списать")
	// ErrSubtaskWarehouseDeleted — количество в строке товара заказа, склад которой мягко удалён,
	// нельзя увеличить (fix-loop 1 части 6): иначе заказ списывал бы новый товар с удалённого
	// склада. Уменьшение, удаление строки и смена склада на действующий разрешены.
	ErrSubtaskWarehouseDeleted = errorspkg.New("WAREHOUSE_DELETED", http.StatusConflict,
		"Склад строки удалён — выберите другой склад")
	ErrPaidAdminOnly = errorspkg.New("PAID_ADMIN_ONLY", http.StatusForbidden, "Оплату меняет только администратор")

	// Sentinel'ы фазы 6 (design-errors.md §Codes 16-22, карточка заказа).
	ErrSubtaskNotFound        = errorspkg.New("SUBTASK_NOT_FOUND", http.StatusNotFound, "Услуга не найдена")
	ErrSubtaskProductNotFound = errorspkg.New("SUBTASK_PRODUCT_NOT_FOUND", http.StatusNotFound, "Товар услуги не найден")
	ErrSubtaskLogNotFound     = errorspkg.New("SUBTASK_LOG_NOT_FOUND", http.StatusNotFound, "Запись журнала не найдена")
	ErrTaskEquipmentNotFound  = errorspkg.New("TASK_EQUIPMENT_NOT_FOUND", http.StatusNotFound, "Оборудование не найдено в заказе")
	ErrTaskEquipmentInUse     = errorspkg.New("TASK_EQUIPMENT_IN_USE", http.StatusConflict, "Оборудование выбрано в услугах заказа — сначала смените его в услугах")
	ErrSubtaskFinalColumn     = errorspkg.New("SUBTASK_FINAL_COLUMN", http.StatusConflict, "Заказ в финальной колонке: нельзя добавить услугу или вернуть её из «Выполнена»")
	ErrSubtaskLogForbidden    = errorspkg.New("SUBTASK_LOG_FORBIDDEN", http.StatusForbidden, "Можно менять только свои записи")

	// ErrSubtaskExecutorPaid — смена/снятие исполнителя у услуги с отметкой «Выплачено
	// исполнителю» (D66): сначала снять отметку.
	ErrSubtaskExecutorPaid = errorspkg.New("SUBTASK_EXECUTOR_PAID", http.StatusConflict, "Услуга уже выплачена исполнителю — сначала снимите отметку о выплате")

	// ErrEquipmentInTransfer — единица стоит в незавершённом документе «Перемещение»
	// (transfers-core DS-026): убрать её из заказа нельзя; текст с номером документа и
	// details.transfer_id собирает equipmentInTransfer (копия, sentinel не мутируется).
	ErrEquipmentInTransfer = errorspkg.New("EQUIPMENT_IN_TRANSFER", http.StatusConflict, "Оборудование в перемещении — сначала отмените его")
	ErrNoActiveWarehouse   = errorspkg.New("NO_ACTIVE_WAREHOUSE", http.StatusConflict, "Нет ни одного активного склада — добавьте склад в справочнике «Склады»")

	// Sentinel'ы фазы 7 (вложения заказа, D21). ATTACHMENT_TOO_LARGE — текст по виду файла
	// собирает attachmentTooLarge (копия, sentinel не мутируется).
	ErrAttachmentNotFound        = errorspkg.New("ATTACHMENT_NOT_FOUND", http.StatusNotFound, "Вложение не найдено")
	ErrAttachmentForbidden       = errorspkg.New("ATTACHMENT_FORBIDDEN", http.StatusForbidden, "Удалить вложение может только автор или администратор")
	ErrAttachmentTypeUnsupported = errorspkg.New("ATTACHMENT_TYPE_UNSUPPORTED", http.StatusBadRequest, "Неподдерживаемый тип файла: можно фото (JPEG, PNG, GIF, WebP), видео (MP4, WebM, MOV) или PDF")
	ErrAttachmentTooLarge        = errorspkg.New("ATTACHMENT_TOO_LARGE", http.StatusRequestEntityTooLarge, "Файл слишком большой")
)

// ErrNotDeleted — 409 NOT_DELETED «Запись уже восстановлена» (soft-delete-core DS-024): код
// общий для всех видов записей, определён в пакете softdelete.
var ErrNotDeleted = softdelete.ErrNotDeleted

// attachmentTooLarge — 413 с понятным текстом по виду файла и details.max_bytes.
func attachmentTooLarge(kind string) *errorspkg.AppError {
	e := ErrAttachmentTooLarge.WithDetail("max_bytes", attachmentLimit(kind))
	e.Message = attachmentLimitMessage(kind)
	return e
}

// equipmentInTransfer — 409 EQUIPMENT_IN_TRANSFER с номером мешающего документа.
func equipmentInTransfer(transferID int64) *errorspkg.AppError {
	e := ErrEquipmentInTransfer.WithDetail("transfer_id", transferID)
	e.Message = fmt.Sprintf("Оборудование в перемещении #%d — сначала отмените его", transferID)
	return e
}

// Внутренние (репозиторные) ошибки — наружу не текут, сервисный слой мапит их через
// errors.Is в sentinel'ы выше.
var (
	errNotFound = errors.New("board: not found")
	// errColumnHasTasks — 23503/23001 на tasks_column_id_fkey при DELETE columns (в т.ч.
	// гонка: заказ создан между EXISTS-проверкой сервиса и DELETE).
	errColumnHasTasks = errors.New("board: column has tasks")
	// errColumnDeleted — колонка из тела запроса мягко удалена (TaskRepo.LockTargetColumn):
	// наружу — 404 COLUMN_NOT_FOUND (soft-delete-board-columns DS-026).
	errColumnDeleted = errors.New("board: column deleted")
	// errVersionConflict — UPDATE tasks ... WHERE version=$n вернул 0 строк при уже
	// подтверждённом (LockTask) существовании заказа (ErrTaskVersionConflict, AC-010).
	errVersionConflict = errors.New("board: task version conflict")
	// errTaskMovedRetry — внутренняя, наружу не течёт (ADR-100): колонка заказа сменилась
	// между чтением column_id и LockTask → withTx повторяет попытку как 40P01/40001.
	errTaskMovedRetry = errors.New("board: task moved during lock, retry")
)
