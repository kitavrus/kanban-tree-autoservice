package board

import (
	"bytes"
	"encoding/json"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// OptionalAmount — денежное поле запроса с тремя состояниями (D65): поля в теле нет
// (Set=false — значение не менять), явный null (Set=true, Value=nil — очистить), число
// (Set=true, Value — установить). Обычный *numeric.Amount «нет поля» и null не различает.
// encoding/json вызывает UnmarshalJSON только для присутствующего поля, в т.ч. для null.
type OptionalAmount struct {
	Set   bool
	Value *numeric.Amount
}

func (o *OptionalAmount) UnmarshalJSON(b []byte) error {
	o.Set = true
	o.Value = nil
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return nil
	}
	var a numeric.Amount
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	o.Value = &a
	return nil
}
