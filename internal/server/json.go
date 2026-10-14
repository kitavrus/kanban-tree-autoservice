package server

import (
	"bytes"
	"encoding/json"
	"errors"
)

// strictJSON — декодер тела запросов: неизвестные поля и мусор после объекта → ошибка
// (хендлер отвечает 400, §TD-4 «валидация/мусор → 400, не 500»).
func strictJSON(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("json: unexpected data after object")
	}
	return nil
}
