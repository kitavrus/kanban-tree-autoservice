package catalog

import "github.com/kitavrus/kanban-tree/pkg/numeric"

// Amount — алиас на pkg/numeric.Amount (ADR-101): тип и его Marshal/UnmarshalJSON/String
// перенесены в общий пакет, т.к. используются и другими фичами (board), не только catalog.
// Код-потребитель catalog не меняется — Amount(...) и catalog.Amount продолжают работать.
type Amount = numeric.Amount
