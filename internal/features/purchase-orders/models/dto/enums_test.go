package dto_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
)

// swag-теги enums на полях DTO совпадают с источником правды constants (Sec 15.1): новый статус
// или источник строки без правки тега роняет тест.
func TestSwagEnumsMatchConstants(t *testing.T) {
	cases := []struct {
		typ   any
		field string
		want  []string
	}{
		{dto.ListItem{}, "Status", constants.Statuses},
		{dto.Order{}, "Status", constants.Statuses},
		{dto.SupplierListItem{}, "Status", constants.SupplierStatuses},
		{dto.SupplierOrder{}, "Status", constants.SupplierStatuses},
		{dto.ItemRequest{}, "Reason", constants.Reasons},
		{dto.OrderItem{}, "Reason", constants.Reasons},
		{dto.Suggestion{}, "Reason", []string{constants.ReasonForOrders, constants.ReasonBelowMin}},
	}
	for _, c := range cases {
		f, ok := reflect.TypeOf(c.typ).FieldByName(c.field)
		if !assert.True(t, ok, "%T.%s", c.typ, c.field) {
			continue
		}
		assert.Equal(t, c.want, strings.Split(f.Tag.Get("enums"), ","), "%T.%s", c.typ, c.field)
	}
}
