package board

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Оборудование заказа и фильтр «Склад» доски — unit-уровень: тело POST /tasks/:id/equipment
// (упразднённый warehouse_id игнорируется, transfers-partners DS-045), enum местоположения
// единицы (transfers-core DS-008), разбор фильтра доски (DS-046), текст 409
// EQUIPMENT_IN_TRANSFER (DS-026).

// TestTS027_TaskEquipmentCreateRequest_WarehouseIDIgnored — хвост TS-027 (transfers-partners
// DS-045): «склад приёма» упразднён — warehouse_id в теле POST /tasks/:id/equipment
// принимается разбором, но не валидируется и не используется при любом значении.
func TestTS027_TaskEquipmentCreateRequest_WarehouseIDIgnored(t *testing.T) {
	for _, body := range []string{
		`{"equipment_id":"` + validUUID + `"}`,
		`{"equipment_id":"` + validUUID + `","warehouse_id":"` + validUUID + `"}`,
		`{"equipment_id":"` + validUUID + `","warehouse_id":"almaty"}`,
		`{"equipment_id":"` + validUUID + `","warehouse_id":""}`,
		`{"equipment_id":"` + validUUID + `","warehouse_id":null}`,
		`{"equipment_id":"` + validUUID + `","warehouse_id":42}`,
		`{"equipment_id":"` + validUUID + `","warehouse_id":{"id":"x"}}`,
	} {
		var in TaskEquipmentCreateRequest
		dec := json.NewDecoder(strings.NewReader(body))
		dec.DisallowUnknownFields() // как строгий разбор тела в роутере
		require.NoError(t, dec.Decode(&in), body)
		assert.Empty(t, in.normalize().validate(), body)
	}
}

// TestEquipmentEnums_MatchMigration — значения совпадают с CHECK миграции 0014 (сверка с
// pg_get_constraintdef — live-сьюта equipment_location_http_live_test.go).
func TestEquipmentEnums_MatchMigration(t *testing.T) {
	assert.ElementsMatch(t, []string{"at_client", "at_warehouse", "in_transit"}, EquipmentLocations)
}

// TestTS018_EquipmentInTransferError — TS-018 (unit): 409 EQUIPMENT_IN_TRANSFER несёт номер
// документа в тексте и details.transfer_id; sentinel не мутируется.
func TestTS018_EquipmentInTransferError(t *testing.T) {
	e := equipmentInTransfer(42)
	assert.Equal(t, "EQUIPMENT_IN_TRANSFER", e.Code)
	assert.Equal(t, 409, e.HTTP)
	assert.Equal(t, "Оборудование в перемещении #42 — сначала отмените его", e.Message)
	assert.Equal(t, map[string]any{"transfer_id": int64(42)}, e.Extra)
	assert.Nil(t, ErrEquipmentInTransfer.Extra)
	assert.NotContains(t, ErrEquipmentInTransfer.Message, "#42")
}

// TestParseTaskFilter_WarehouseID — TS-024 (unit): warehouse_id невалидный → VALIDATION по полю;
// валидный — в фильтре; отсутствует — nil.
func TestParseTaskFilter_WarehouseID(t *testing.T) {
	_, err := parseFilter(t, "warehouse_id=almaty", admin)
	var ae *errorspkg.AppError
	require.True(t, errors.As(err, &ae))
	require.Equal(t, "VALIDATION", ae.Code)
	require.Len(t, ae.Fields, 1)
	assert.Equal(t, "warehouse_id", ae.Fields[0].Name)

	f, err := parseFilter(t, "", admin)
	require.NoError(t, err)
	assert.Nil(t, f.WarehouseID)

	f, err = parseFilter(t, "warehouse_id="+validUUID, admin)
	require.NoError(t, err)
	require.NotNil(t, f.WarehouseID)
	assert.Equal(t, validUUID, *f.WarehouseID)
}
