package board

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNavBadgeCache_TTLAndErrors(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	calls, next := 0, 3
	var loadErr error
	c := newNavBadgeCache(10*time.Second, func(context.Context) (int, error) {
		calls++
		return next, loadErr
	})
	c.now = func() time.Time { return now }
	ctx := context.Background()

	v, err := c.get(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, v)
	require.Equal(t, 1, calls)

	next = 7
	now = now.Add(9 * time.Second)
	v, _ = c.get(ctx)
	require.Equal(t, 3, v, "в пределах TTL — из кэша")
	require.Equal(t, 1, calls)

	now = now.Add(time.Second)
	v, _ = c.get(ctx)
	require.Equal(t, 7, v, "TTL истёк — новое значение")
	require.Equal(t, 2, calls)

	loadErr = errors.New("db down")
	now = now.Add(time.Minute)
	_, err = c.get(ctx)
	require.Error(t, err)
	loadErr = nil
	next = 9
	v, err = c.get(ctx)
	require.NoError(t, err)
	require.Equal(t, 9, v, "ошибка не кэшируется")
	require.Equal(t, 4, calls)

	next = 11
	c.invalidate()
	v, _ = c.get(ctx)
	require.Equal(t, 11, v, "после invalidate — сразу новое значение, TTL не ждём")
	require.Equal(t, 5, calls)
}
