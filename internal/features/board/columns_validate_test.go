package board

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptrInt(v int) *int { return &v }

func TestColumnInputValidate_EmptyName(t *testing.T) {
	in := ColumnInput{Name: "  ", Color: "#94A3B8"}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "name", fields[0].Name)
	assert.Equal(t, msgRequired, fields[0].Message)
}

func TestColumnInputValidate_NameTooLong(t *testing.T) {
	long := make([]byte, columnNameMax+1)
	for i := range long {
		long[i] = 'a'
	}
	in := ColumnInput{Name: string(long), Color: "#94A3B8"}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "name", fields[0].Name)
	assert.Equal(t, msgTooLong, fields[0].Message)
}

func TestColumnInputValidate_InvalidColor(t *testing.T) {
	cases := []string{"", "94A3B8", "#94A3B", "#GGGGGG", "#94a3b8g"}
	for _, c := range cases {
		in := ColumnInput{Name: "Приёмка", Color: c}.normalize()
		fields := in.validate()
		assert.Len(t, fields, 1, "color=%q", c)
		assert.Equal(t, "color", fields[0].Name)
		assert.Equal(t, msgInvalidFormat, fields[0].Message)
	}
}

func TestColumnInputValidate_ValidInput(t *testing.T) {
	in := ColumnInput{
		Name: "Приёмка", Color: "#94A3B8", IsFinal: false, ConsumesStock: true, Position: ptrInt(1),
	}.normalize()
	assert.Empty(t, in.validate())
}

func TestColumnInputValidate_InvalidPosition(t *testing.T) {
	in := ColumnInput{Name: "Приёмка", Color: "#94A3B8", Position: ptrInt(0)}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "position", fields[0].Name)
	assert.Equal(t, msgInvalid, fields[0].Message)
}
