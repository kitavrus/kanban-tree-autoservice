package transfers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Unit-сценарии части transfers-sales (docs/features/transfers-sales/draft-spec.md §11):
// TS-003 — валидатор цены продажи товарной строки накладной «склад → клиент».

func priced(id, qty, price string) ItemInput {
	p := numeric.Amount(price)
	return ItemInput{Kind: KindProduct, ProductID: id, QtySent: numeric.Amount(qty), Price: &p}
}

// invoice — накладная «склад → клиент» со строками items.
func invoice(items ...ItemInput) Input {
	in := validRoute(DirWarehouseToClient)
	in.Items = items
	return in
}

// TS-003: цена допустима только у товара в warehouse_to_client; отрицательная → negative; три
// знака после запятой, не число, больше 9999999999.99 → invalid; в warehouse_to_warehouse,
// supplier_to_warehouse и в строке-единице → not_allowed; не передана → ошибки нет (цену
// каталога подставляет repo при сохранении, DS-039).
func TestTS003_PriceValidator(t *testing.T) {
	ok := []string{"0", "0.00", "0.5", "18500.00", "18500.5", "1", "9999999999.99"}
	for _, p := range ok {
		t.Run("допустимая "+p, func(t *testing.T) {
			assert.Empty(t, invoice(unit(eqX), priced(prodX, "1", p)).normalize().validate())
		})
	}

	bad := map[string]string{
		"-1":             msgNegative,
		"-0.01":          msgNegative,
		"1.999":          msgInvalid,
		"0.001":          msgInvalid,
		"10000000000":    msgInvalid,
		"10000000000.00": msgInvalid,
		"1000000000000":  msgInvalid, // 1e12 (abuse case 1)
		"1e12":           msgInvalid,
		"abc":            msgInvalid,
		"NaN":            msgInvalid,
		"1,5":            msgInvalid,
		"":               msgInvalid, // пустое значение при переданном поле — не число
	}
	for p, want := range bad {
		t.Run("недопустимая "+p, func(t *testing.T) {
			got := fieldMap(invoice(priced(prodX, "1", p)).normalize().validate())
			assert.Equal(t, map[string]string{"items[0].price": want}, got)
		})
	}

	t.Run("не передана — без ошибки", func(t *testing.T) {
		in := invoice(unit(eqX), product(prodX, "2")).normalize()
		assert.Empty(t, in.validate())
		assert.Nil(t, in.Items[1].Price, "normalize цену не выдумывает — её подставит repo из каталога")
	})

	t.Run("normalize сохраняет цену", func(t *testing.T) {
		in := invoice(priced(prodX, "2", "18500.00")).normalize()
		require.NotNil(t, in.Items[0].Price)
		assert.Equal(t, numeric.Amount("18500.00"), *in.Items[0].Price)
	})

	t.Run("накладная только из товара", func(t *testing.T) {
		assert.Empty(t, invoice(priced(prodX, "3", "100")).normalize().validate())
	})

	// цена вне накладной клиенту → not_allowed (abuse case 2), в т.ч. нулевая.
	for _, d := range []string{DirWarehouseToWarehouse, DirSupplierToWarehouse} {
		for _, p := range []string{"100", "0"} {
			t.Run("not_allowed "+d+" "+p, func(t *testing.T) {
				in := validRoute(d)
				in.Items = []ItemInput{priced(prodX, "1", p)}
				assert.Equal(t, map[string]string{"items[0].price": msgNotAllowed}, fieldMap(in.normalize().validate()))
			})
		}
	}

	t.Run("not_allowed в строке-единице", func(t *testing.T) {
		p := numeric.Amount("100")
		for _, d := range []string{DirWarehouseToClient, DirClientToWarehouse, DirWarehouseToWarehouse} {
			in := validRoute(d)
			in.Items = []ItemInput{{Kind: KindEquipment, EquipmentID: eqX, Price: &p}}
			assert.Equal(t, map[string]string{"items[0].price": msgNotAllowed}, fieldMap(in.normalize().validate()), d)
		}
	})

	// товар в приёме от клиента по-прежнему недопустим — ошибка kind, цена не разбирается.
	t.Run("client_to_warehouse: товар с ценой — kind not_allowed", func(t *testing.T) {
		in := validRoute(DirClientToWarehouse)
		in.Items = []ItemInput{priced(prodX, "1", "-5")}
		assert.Equal(t, map[string]string{"items[0].kind": msgNotAllowed}, fieldMap(in.normalize().validate()))
	})

	t.Run("ошибки цены и количества — обе", func(t *testing.T) {
		got := fieldMap(invoice(priced(prodX, "0", "-1")).normalize().validate())
		assert.Equal(t, map[string]string{"items[0].qty_sent": msgNotPositive, "items[0].price": msgNegative}, got)
	})
}

// TS-003 (wire): price — JSON-число; нет поля и null → nil (цена каталога); строка → ошибка
// разбора тела (400), как у остальных числовых полей проекта.
func TestTS003_PriceWire(t *testing.T) {
	var it ItemInput
	require.NoError(t, json.Unmarshal([]byte(`{"kind":"product","product_id":"`+prodX+`","qty_sent":1,"price":18500.50}`), &it))
	require.NotNil(t, it.Price)
	assert.Equal(t, numeric.Amount("18500.50"), *it.Price, "без потери точности и хвостовых нулей")

	it = ItemInput{}
	require.NoError(t, json.Unmarshal([]byte(`{"kind":"product","product_id":"`+prodX+`","qty_sent":1,"price":null}`), &it))
	assert.Nil(t, it.Price, "null — цена каталога")

	it = ItemInput{}
	require.NoError(t, json.Unmarshal([]byte(`{"kind":"product","product_id":"`+prodX+`","qty_sent":1}`), &it))
	assert.Nil(t, it.Price, "нет поля — цена каталога")

	assert.Error(t, json.Unmarshal([]byte(`{"kind":"product","price":"abc"}`), &it), "строка — не число")
	assert.Error(t, json.Unmarshal([]byte(`{"kind":"product","price":"18500.00"}`), &it), "число в кавычках — не число")
}

// TS-003: правка цены — та же правка заявки: вне статуса «Заявка» → 409 со своим сообщением.
func TestTS003_PriceEditStatusConflict(t *testing.T) {
	for status, msg := range map[string]string{
		StatusInTransit: "Перемещение уже отправлено — править нельзя",
		StatusReceived:  "Перемещение уже получено — править нельзя",
		StatusCancelled: "Перемещение отменено — править нельзя",
	} {
		e := statusConflict(actionUpdate, status)
		assert.Equal(t, codeStatusConflict, e.Code, status)
		assert.Equal(t, msg, e.Message, status)
	}
}

// TS-014 (unit): отправка списывает товар со склада отправления у «склад → склад» и накладной
// «склад → клиент»; цена — только у накладной.
func TestTS014_SendsStockAndPricedDirections(t *testing.T) {
	for d, want := range map[string]bool{
		DirWarehouseToWarehouse: true, DirWarehouseToClient: true,
		DirClientToWarehouse: false, DirSupplierToWarehouse: false,
	} {
		assert.Equal(t, want, sendsStock(d), d)
	}
	for _, d := range Directions {
		assert.Equal(t, d == DirWarehouseToClient, pricedDirection(d), d)
	}
}
