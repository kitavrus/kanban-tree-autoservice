package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/validators"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

var (
	admin    = Actor{ID: "a", Role: RoleAdmin}
	execA    = Actor{ID: "ea", Role: RoleExecutor}
	execB    = Actor{ID: "eb", Role: RoleExecutor}
	supplier = "s1"
)

func head(status string, opts ...func(*models.Head)) models.Head {
	sup := supplier
	h := models.Head{ID: "o", Status: status, Version: 5, CreatedBy: execA.ID, SupplierID: &sup, ItemsCount: 2, SupplierAvailable: true}
	for _, o := range opts {
		o(&h)
	}
	return h
}

func bazaar(h *models.Head)   { h.SupplierID = nil }
func empty(h *models.Head)    { h.ItemsCount = 0 }
func hasLogin(h *models.Head) { h.SupplierHasLogin = true }
func archived(h *models.Head) { h.SupplierAvailable = false }

// TS-003 (DS-011, DS-012, DS-042, DS-075): матрица переходов §5 на уровне правил — каждый
// разрешённый переход проходит, каждый запрещённый даёт свой 409.
func TestTS003_TransitionMatrix(t *testing.T) {
	type tc struct {
		name  string
		check func(models.Head) error
		h     models.Head
		want  *errorspkg.AppError // nil — ok
	}
	cases := []tc{
		{"send draft", SendCheck(admin, 5), head("draft"), nil},
		{"send sent", SendCheck(admin, 5), head("sent"), errorspkg.ErrPurchaseOrderAlreadySent},
		{"send ready", SendCheck(admin, 5), head("ready"), errorspkg.ErrPurchaseOrderSendNotDraft},
		{"send picked_up", SendCheck(admin, 5), head("picked_up"), errorspkg.ErrPurchaseOrderSendNotDraft},
		{"send cancelled", SendCheck(admin, 5), head("cancelled"), errorspkg.ErrPurchaseOrderSendNotDraft},
		{"send bazaar", SendCheck(admin, 5), head("draft", bazaar), errorspkg.ErrPurchaseOrderNoSupplier},
		{"send empty", SendCheck(admin, 5), head("draft", empty), errorspkg.ErrPurchaseOrderEmpty},
		{"send stale", SendCheck(admin, 4), head("draft"), errorspkg.ErrPurchaseOrderVersionConflict},

		{"recall sent", RecallCheck(admin, 5), head("sent"), nil},
		{"recall ready", RecallCheck(admin, 5), head("ready"), errorspkg.ErrPurchaseOrderRecallReady},
		{"recall draft", RecallCheck(admin, 5), head("draft"), errorspkg.ErrPurchaseOrderRecallNotSent},
		{"recall picked_up", RecallCheck(admin, 5), head("picked_up"), errorspkg.ErrPurchaseOrderRecallNotSent},
		{"recall stale", RecallCheck(admin, 1), head("sent"), errorspkg.ErrPurchaseOrderVersionConflict},

		{"pick-up ready", PickUpCheck(admin, 5), head("ready"), nil},
		{"pick-up bazaar draft", PickUpCheck(admin, 5), head("draft", bazaar), nil},
		{"pick-up bazaar empty", PickUpCheck(admin, 5), head("draft", bazaar, empty), errorspkg.ErrPurchaseOrderEmpty},
		{"pick-up supplier draft", PickUpCheck(admin, 5), head("draft"), errorspkg.ErrPurchaseOrderNotReadySendFirst},
		{"pick-up sent no login", PickUpCheck(admin, 5), head("sent"), nil},
		{"pick-up sent with login", PickUpCheck(admin, 5), head("sent", hasLogin), errorspkg.ErrPurchaseOrderNotReadyWait},
		{"pick-up picked_up", PickUpCheck(admin, 5), head("picked_up"), errorspkg.ErrPurchaseOrderAlreadyClosed},
		{"pick-up cancelled", PickUpCheck(admin, 5), head("cancelled"), errorspkg.ErrPurchaseOrderIsCancelled},
		{"pick-up stale", PickUpCheck(admin, 6), head("ready"), errorspkg.ErrPurchaseOrderVersionConflict},

		{"cancel draft", CancelCheck(admin, 5), head("draft"), nil},
		{"cancel sent", CancelCheck(admin, 5), head("sent"), nil},
		{"cancel ready", CancelCheck(admin, 5), head("ready"), nil},
		{"cancel picked_up", CancelCheck(admin, 5), head("picked_up"), errorspkg.ErrPurchaseOrderCancelClosed},
		{"cancel cancelled", CancelCheck(admin, 5), head("cancelled"), errorspkg.ErrPurchaseOrderAlreadyCancelled},
		{"cancel stale", CancelCheck(admin, 2), head("draft"), errorspkg.ErrPurchaseOrderVersionConflict},

		{"delete draft", DeleteCheck(5), head("draft"), nil},
		{"delete sent", DeleteCheck(5), head("sent"), errorspkg.ErrPurchaseOrderDeleteNotDraft},
		{"delete stale", DeleteCheck(4), head("draft"), errorspkg.ErrPurchaseOrderVersionConflict},

		// DS-051/DS-060: executor трогает только свои — чужой → 404.
		{"executor own send", SendCheck(execA, 5), head("draft"), nil},
		{"executor foreign send", SendCheck(execB, 5), head("draft"), errorspkg.ErrPurchaseOrderNotFound},
		{"executor foreign cancel", CancelCheck(execB, 5), head("draft"), errorspkg.ErrPurchaseOrderNotFound},
		{"executor foreign recall", RecallCheck(execB, 5), head("sent"), errorspkg.ErrPurchaseOrderNotFound},
		{"executor foreign pick-up", PickUpCheck(execB, 5), head("ready"), errorspkg.ErrPurchaseOrderNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.check(c.h)
			if c.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.Same(t, c.want, err)
		})
	}
}

// TS-005 (негатив) / §6.4: правка в ready/picked_up/cancelled — 409 READ_ONLY с текстом по
// статусу; чужой id строки — 400; устаревшая версия — 409.
func TestUpdateCheck(t *testing.T) {
	lines := []validators.Line{{Index: 0}}
	lines[0].ID, lines[0].ProductID = "i1", "p1"
	existing := []models.ExistingItem{{ID: "i1", ProductID: "p1"}}
	assert.NoError(t, UpdateCheck(admin, 5, lines)(head("sent"), existing))
	assert.Same(t, errorspkg.ErrPurchaseOrderReadOnlyReady, UpdateCheck(admin, 5, lines)(head("ready"), existing))
	assert.Same(t, errorspkg.ErrPurchaseOrderReadOnlyClosed, UpdateCheck(admin, 5, lines)(head("picked_up"), existing))
	assert.Same(t, errorspkg.ErrPurchaseOrderReadOnlyCancelled, UpdateCheck(admin, 5, lines)(head("cancelled"), existing))
	assert.Same(t, errorspkg.ErrPurchaseOrderVersionConflict, UpdateCheck(admin, 4, lines)(head("draft"), existing))
	assert.Same(t, errorspkg.ErrPurchaseOrderNotFound, UpdateCheck(execB, 5, lines)(head("draft"), existing))

	err := UpdateCheck(admin, 5, lines)(head("draft"), []models.ExistingItem{{ID: "i2", ProductID: "p1"}})
	app, ok := err.(*errorspkg.AppError)
	if assert.True(t, ok) {
		assert.Equal(t, "VALIDATION", app.Code)
		assert.Equal(t, "items[0].id", app.Fields[0].Name)
	}
}

func s(v string) *string { return &v }

// TS-006 (DS-038): «Собрано» — строка без «Соберу» → 409 с details.item_ids; qty > 0 без цены
// → 409; все ответы (включая 0 без цены) → ok. TS-002 негатив: ответ в ready → 409 READ_ONLY.
func TestTS006_ReadyCheck(t *testing.T) {
	chk := ReadyCheck()
	err := chk(head("sent"), []models.AnswerState{{ItemID: "1", SupplierQty: s("1"), SupplierPrice: s("10")}, {ItemID: "2"}})
	app := err.(*errorspkg.AppError)
	assert.Equal(t, errorspkg.CodePurchaseOrderItemsIncomplete, app.Code)
	assert.Equal(t, errorspkg.ErrPurchaseOrderItemsNoQty.Message, app.Message)
	assert.Equal(t, []string{"2"}, app.Extra["item_ids"])
	assert.Nil(t, errorspkg.ErrPurchaseOrderItemsNoQty.Extra, "sentinel не мутирован")

	err = chk(head("sent"), []models.AnswerState{{ItemID: "1", SupplierQty: s("2.000")}})
	app = err.(*errorspkg.AppError)
	assert.Equal(t, errorspkg.ErrPurchaseOrderItemsNoPrice.Message, app.Message)
	assert.Equal(t, []string{"1"}, app.Extra["item_ids"])

	assert.NoError(t, chk(head("sent"), []models.AnswerState{{ItemID: "1", SupplierQty: s("0.000")}, {ItemID: "2", SupplierQty: s("3"), SupplierPrice: s("0")}}))
	assert.Same(t, errorspkg.ErrPurchaseOrderAlreadyReady, chk(head("ready"), nil))
	assert.Same(t, errorspkg.ErrPurchaseOrderAlreadyPickedUp, chk(head("picked_up"), nil))
	assert.Same(t, errorspkg.ErrPurchaseOrderNotFound, chk(head("draft"), nil))
	assert.Same(t, errorspkg.ErrPurchaseOrderNotFound, chk(head("cancelled"), nil))

	ans := AnswerCheck(3)
	assert.NoError(t, ans(head("sent"), models.AnswerState{Version: 3}))
	assert.Same(t, errorspkg.ErrPurchaseItemVersionConflict, ans(head("sent"), models.AnswerState{Version: 4}))
	assert.Same(t, errorspkg.ErrPurchaseAnswerReadOnlyReady, ans(head("ready"), models.AnswerState{Version: 3}))
	assert.Same(t, errorspkg.ErrPurchaseAnswerReadOnlyPickedUp, ans(head("picked_up"), models.AnswerState{Version: 3}))
	assert.Same(t, errorspkg.ErrPurchaseAnswerReadOnlyRecalled, ans(head("draft"), models.AnswerState{Version: 3}))
	assert.Same(t, errorspkg.ErrPurchaseAnswerReadOnlyRecalled, ans(head("cancelled"), models.AnswerState{Version: 3}))
}

func TestActorOwner(t *testing.T) {
	assert.Equal(t, "", admin.owner())
	assert.Equal(t, "ea", execA.owner())
	assert.Error(t, staffOnly(Actor{Role: RoleSupplier}))
	assert.NoError(t, staffOnly(execA))
	assert.Error(t, supplierOnly(Actor{Role: RoleSupplier}), "supplier без привязки")
	assert.NoError(t, supplierOnly(Actor{Role: RoleSupplier, SupplierID: "x"}))
}

// DS-043: отправка черновика архивному/удалённому поставщику — 400 supplier_id: unavailable,
// как при создании (§6.2).
func TestSendCheck_SupplierUnavailable(t *testing.T) {
	err := SendCheck(admin, 5)(head("draft", archived))
	app, ok := err.(*errorspkg.AppError)
	if assert.True(t, ok) {
		assert.Equal(t, "VALIDATION", app.Code)
		assert.Equal(t, "supplier_id", app.Fields[0].Name)
		assert.Equal(t, "Поставщик недоступен", app.Message)
	}
}
