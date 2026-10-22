// Package mappers — конвертация строк БД (models) в wire-типы API (dto). Время — UTC
// (DS-033): показ в поясе смотрящего делает клиент (KT.time) и PDF (timezone.Format).
package mappers

import (
	"time"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
)

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// RecipientName — поставщик или «Базар (сами)» (§6.1).
func RecipientName(o *models.PurchaseOrder) string {
	if o.SupplierID == nil {
		return constants.BazaarName
	}
	return o.SupplierName
}

// ToListItem — строка списка §6.1.
func ToListItem(o *models.PurchaseOrder) dto.ListItem {
	return dto.ListItem{
		ID: o.ID, Number: o.Number, NumberDisplay: constants.NumberDisplay(o.Number),
		SupplierID: o.SupplierID, RecipientName: RecipientName(o), Status: o.Status,
		ItemsCount: o.ItemsCount, TotalAmount: o.TotalAmount, CreatedByName: o.CreatedByName,
		CreatedAt: o.CreatedAt.UTC(), SentAt: utc(o.SentAt), PickedUpAt: utc(o.PickedUpAt),
		HasDiscrepancy: o.HasDiscrepancy, IsChanged: o.IsChanged,
	}
}

// ToOrder — карточка §6.3. can_edit — статус draft/sent (чужие executor'у не отдаются вовсе).
func ToOrder(o *models.PurchaseOrder) *dto.Order {
	out := &dto.Order{
		ID: o.ID, Number: o.Number, NumberDisplay: constants.NumberDisplay(o.Number),
		Status: o.Status, Version: o.Version, SupplierID: o.SupplierID,
		RecipientName: RecipientName(o), RecipientPhone: o.SupplierPhone,
		SupplierHasLogin: o.SupplierHasLogin, Note: o.Note,
		CanEdit:       o.Status == constants.StatusDraft || o.Status == constants.StatusSent,
		CreatedByName: o.CreatedByName, CreatedAt: o.CreatedAt.UTC(),
		SentAt: utc(o.SentAt), ReadyAt: utc(o.ReadyAt), PickedUpAt: utc(o.PickedUpAt),
		CancelledAt: utc(o.CancelledAt), CancelReason: o.CancelReason,
		Items: make([]dto.OrderItem, 0, len(o.Items)), TotalAmount: o.TotalAmount,
	}
	if o.SupplierID == nil {
		out.RecipientPhone = ""
	}
	for i := range o.Items {
		it := &o.Items[i]
		out.Items = append(out.Items, dto.OrderItem{
			ID: it.ID, ProductID: it.ProductID, Name: it.Name, Sku: it.Sku, Unit: it.Unit,
			Qty: it.Qty, Reason: it.Reason, SupplierQty: it.SupplierQty, SupplierPrice: it.SupplierPrice,
			SupplierNote: it.SupplierNote, LineAmount: it.LineAmount, HasDiscrepancy: it.HasDiscrepancy,
			CustomerChanged: it.CustomerChanged, Version: it.Version,
		})
	}
	return out
}

// ToSupplierListItem — §6.12.
func ToSupplierListItem(o *models.PurchaseOrder) dto.SupplierListItem {
	return dto.SupplierListItem{
		ID: o.ID, NumberDisplay: constants.NumberDisplay(o.Number), Status: o.Status,
		SentAt: utc(o.SentAt), ItemsCount: o.ItemsCount, AnsweredCount: o.AnsweredCount,
		TotalAmount: o.TotalAmount, IsChanged: o.IsChanged,
	}
}

// ToSupplierItem — строка кабинета §6.13 (без reason/product_id, DS-046).
func ToSupplierItem(it *models.PurchaseOrderItem) dto.SupplierItem {
	return dto.SupplierItem{
		ID: it.ID, Name: it.Name, Sku: it.Sku, Unit: it.Unit, Qty: it.Qty,
		SupplierQty: it.SupplierQty, SupplierPrice: it.SupplierPrice, SupplierNote: it.SupplierNote,
		LineAmount: it.LineAmount, CustomerChanged: it.CustomerChanged, Version: it.Version,
	}
}

// ToSupplierOrder — карточка кабинета §6.13.
func ToSupplierOrder(o *models.PurchaseOrder, firmName, firmPhone string) *dto.SupplierOrder {
	out := &dto.SupplierOrder{
		ID: o.ID, NumberDisplay: constants.NumberDisplay(o.Number), Status: o.Status,
		SentAt: utc(o.SentAt), Note: o.Note, NoteChanged: o.NoteChanged,
		FirmName: firmName, FirmPhone: firmPhone, CanAnswer: o.Status == constants.StatusSent,
		Items: make([]dto.SupplierItem, 0, len(o.Items)), TotalAmount: o.TotalAmount,
	}
	for i := range o.Items {
		out.Items = append(out.Items, ToSupplierItem(&o.Items[i]))
	}
	return out
}

// ToSuggestion — строка дефицита §6.10; checked = false, если запчасть уже в открытом заказе
// (DS-004, DS-049).
func ToSuggestion(s *models.Suggestion) dto.Suggestion {
	out := dto.Suggestion{
		ProductID: s.ProductID, Name: s.Name, Sku: s.Sku, Unit: s.Unit, Stock: s.Stock,
		MinStock: s.MinStock, Pending: s.Pending, ToOrder: s.ToOrder, Reason: s.Reason,
		Tasks: []dto.SuggestionTask{}, InOpenOrders: []dto.OpenOrderRef{},
		Checked: len(s.InOpenOrders) == 0,
	}
	if s.Reason == constants.ReasonForOrders {
		for _, t := range s.Tasks {
			out.Tasks = append(out.Tasks, dto.SuggestionTask{ID: t.ID, Title: t.Title})
		}
	}
	for _, r := range s.InOpenOrders {
		out.InOpenOrders = append(out.InOpenOrders, dto.OpenOrderRef{ID: r.ID, NumberDisplay: constants.NumberDisplay(r.Number), Qty: r.Qty})
	}
	return out
}
