package clients

import (
	"context"
	"errors"

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
// Клиент — FOR KEY SHARE (не удалён), как у POST /clients/:id/equipment.
func CreateEquipmentTx(ctx context.Context, tx pgx.Tx, clientID string, in EquipmentInput) (string, error) {
	id, err := insertEquipment(ctx, tx, clientID, in)
	if errors.Is(err, errClientGone) {
		return "", ErrOrderClientGone
	}
	return id, err
}
