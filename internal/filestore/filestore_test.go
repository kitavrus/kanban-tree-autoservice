package filestore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStore(t *testing.T) *LocalFileStore {
	t.Helper()
	s, err := NewLocal(filepath.Join(t.TempDir(), "nested", "uploads"))
	require.NoError(t, err)
	return s
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}

func TestLocalFileStore_PutOpenDelete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	key := NewKey()

	n, err := s.Put(ctx, key, strings.NewReader("hello"), 5)
	require.NoError(t, err)
	assert.EqualValues(t, 5, n)
	assert.Equal(t, []string{key}, entries(t, s.Dir()), "только целевой файл, без временных")

	f, info, err := s.Open(ctx, key)
	require.NoError(t, err)
	assert.EqualValues(t, 5, info.Size)
	_, err = f.Seek(1, io.SeekStart)
	require.NoError(t, err)
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	assert.Equal(t, "ello", string(b))

	require.NoError(t, s.Delete(ctx, key))
	require.NoError(t, s.Delete(ctx, key), "повторное удаление — не ошибка")
	_, _, err = s.Open(ctx, key)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestLocalFileStore_TooLarge(t *testing.T) {
	s := newStore(t)
	_, err := s.Put(context.Background(), NewKey(), strings.NewReader("123456"), 5)
	assert.ErrorIs(t, err, ErrTooLarge)
	assert.Empty(t, entries(t, s.Dir()), "частичный файл удалён")
}

func TestLocalFileStore_InvalidKey(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for _, k := range []string{"", "../etc/passwd", "a/b", "x.jpg"} {
		_, err := s.Put(ctx, k, strings.NewReader("x"), 10)
		assert.ErrorIs(t, err, ErrInvalidKey, k)
		_, _, err = s.Open(ctx, k)
		assert.ErrorIs(t, err, ErrInvalidKey, k)
		assert.ErrorIs(t, s.Delete(ctx, k), ErrInvalidKey, k)
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestLocalFileStore_ReaderErrorCleansUp(t *testing.T) {
	s := newStore(t)
	_, err := s.Put(context.Background(), NewKey(), failReader{}, 10)
	require.Error(t, err)
	assert.Empty(t, entries(t, s.Dir()))
}

func TestLocalFileStore_CanceledContext(t *testing.T) {
	s := newStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Put(ctx, NewKey(), strings.NewReader("abc"), 10)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, entries(t, s.Dir()))
}
