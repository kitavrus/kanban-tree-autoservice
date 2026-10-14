package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Атомарное создание заказа по госномеру (board, POST /api/v1/tasks с new_client /
// new_equipment): клиент и автомобиль пишутся в ЧУЖОЙ транзакции board — по тем же правилам
// нормализации/валидации и с теми же блокировками, что POST /api/v1/clients и
// POST /api/v1/clients/:id/equipment. Имена полей ошибок — без префикса; вызывающий
// добавляет свой («new_client.», «new_equipment.»).

// Ошибки CreateEquipmentTx, которые вызывающий маппит в поля своей формы.
var (
	// ErrOrderClientGone — клиент не существует либо мягко удалён (в т.ч. параллельно).
	ErrOrderClientGone = errors.New("clients: client not found")
	// ErrOrderTypeNotFound — тип оборудования не существует либо мягко удалён.
	ErrOrderTypeNotFound = errors.New("clients: equipment type not found")
	// ErrOrderTypeArchived — тип в архиве: новая единица только с активным типом (ADR-004).
	ErrOrderTypeArchived = errors.New("clients: equipment type archived")
)

// PrepareClient — normalize + validate карточки клиента (правила POST /api/v1/clients).
func PrepareClient(in ClientInput) (ClientInput, []errorspkg.Field) {
	in = in.normalize()
	return in, in.validate()
}

// PrepareEquipment — normalize + validate единицы (правила POST /clients/:id/equipment).
func PrepareEquipment(in EquipmentInput) (EquipmentInput, []errorspkg.Field) {
	in = in.normalize()
	return in, in.validate()
}

// CreateClientTx — клиент + телефоны в транзакции вызывающего; in — после PrepareClient.
func CreateClientTx(ctx context.Context, tx pgx.Tx, in ClientInput) (string, error) {
	c, err := insertClient(ctx, tx, in)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

// CreateEquipmentTx — единица клиента в транзакции вызывающего; in — после PrepareEquipment.
// Тип проверяется под FOR SHARE (не удалён и активен), клиент — FOR KEY SHARE (не удалён):
// порядок блокировок «клиент → тип», как у POST /clients/:id/equipment.
func CreateEquipmentTx(ctx context.Context, tx pgx.Tx, clientID string, in EquipmentInput) (string, error) {
	var active bool
	err := tx.QueryRow(ctx, `SELECT is_active FROM equipment_types WHERE id = $1::uuid AND deleted_at IS NULL`, in.TypeID).Scan(&active)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", ErrOrderTypeNotFound
	case err != nil:
		return "", err
	case !active:
		return "", ErrOrderTypeArchived
	}
	specsJSON, err := json.Marshal(in.Specs)
	if err != nil {
		return "", fmt.Errorf("clients: marshal specs: %w", err)
	}
	id, err := insertEquipment(ctx, tx, clientID, in, specsJSON)
	switch {
	case errors.Is(err, errClientGone):
		return "", ErrOrderClientGone
	case errors.Is(err, errTypeGone):
		return "", ErrOrderTypeNotFound
	}
	return id, err
}
