package softdelete

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// soft-delete-transfers §6.3, §8.2: подзаголовок строки удалённого документа — «направление ·
// статус» подписями экрана «Перемещения»; неизвестное значение отдаётся как есть.
func TestTransferSubtitle(t *testing.T) {
	for _, tc := range []struct{ direction, status, want string }{
		{"warehouse_to_warehouse", "requested", "Склад → склад · Заявка"},
		{"client_to_warehouse", "cancelled", "Приём от клиента · Отменён"},
		{"warehouse_to_client", "requested", "Возврат клиенту · Заявка"},
		{"supplier_to_warehouse", "cancelled", "Приход от поставщика · Отменён"},
		{"unknown_direction", "in_transit", "unknown_direction · in_transit"},
	} {
		assert.Equal(t, tc.want, TransferSubtitle(tc.direction, tc.status), tc.direction+"/"+tc.status)
	}
}

// Вид transfers зарегистрирован в реестре и получает ключ счётчика «transfers».
func TestKindTransfersRegistry(t *testing.T) {
	assert.True(t, IsKind("transfers"))
	assert.Equal(t, "transfers", KindTransfers.CountKey())
	assert.Equal(t, KindTransfers, Kinds[1], "вкладка «Перемещения» — вторая после «Заказы»")
}
