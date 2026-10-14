package softdelete

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// soft-delete-board-columns §6.3, §8.2: подзаголовок строки удалённой колонки — «финальная»
// только у финальной колонки, иначе пустая строка.
func TestColumnSubtitle(t *testing.T) {
	assert.Equal(t, "финальная", ColumnSubtitle(true))
	assert.Equal(t, "", ColumnSubtitle(false))
}

// Вид columns зарегистрирован в реестре и получает ключ счётчика «columns».
func TestKindColumnsRegistry(t *testing.T) {
	assert.True(t, IsKind("columns"))
	assert.Equal(t, "columns", KindColumns.CountKey())
	assert.Equal(t, KindColumns, Kinds[1], "вкладка «Колонки» — вторая в реестре")
}
