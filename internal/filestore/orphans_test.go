package filestore

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIndex — KeyIndex в памяти: alive — ключи «со строкой в БД»; err — сбой БД.
type fakeIndex struct {
	mu      sync.Mutex
	alive   map[string]struct{}
	err     error
	batches []int
}

func (f *fakeIndex) ExistingKeys(_ context.Context, keys []string) (map[string]struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches = append(f.batches, len(keys))
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]struct{}{}
	for _, k := range keys {
		if _, ok := f.alive[k]; ok {
			out[k] = struct{}{}
		}
	}
	return out, nil
}

func (f *fakeIndex) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.batches)
}

var (
	sweepNow = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	quiet    = slog.New(slog.NewTextHandler(io.Discard, nil))
)

// putAged — файл в каталоге хранилища с заданным возрастом относительно sweepNow.
func putAged(t *testing.T, s *LocalFileStore, name string, age time.Duration) {
	t.Helper()
	p := filepath.Join(s.Dir(), name)
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
	mt := sweepNow.Add(-age)
	require.NoError(t, os.Chtimes(p, mt, mt))
}

func sorted(names []string) []string {
	sort.Strings(names)
	return names
}

func TestSweepOrphans_RemovesOnlyOldUnknownKeysAndTemps(t *testing.T) {
	s := newStore(t)
	aliveOld, orphanOld, orphanYoung := NewKey(), NewKey(), NewKey()
	putAged(t, s, aliveOld, 3*time.Hour)
	putAged(t, s, orphanOld, 2*time.Hour)
	putAged(t, s, orphanYoung, 10*time.Minute)
	putAged(t, s, ".upload-111", 2*time.Hour)   // брошенный temp после падения
	putAged(t, s, ".upload-222", 5*time.Minute) // идущая загрузка
	putAged(t, s, "readme.txt", 48*time.Hour)   // чужой файл — не ключ и не temp
	// неканонические формы uuid (uuid.Validate их принимает) — не ключи NewKey, не трогаются
	nonCanon := []string{
		strings.ReplaceAll(NewKey(), "-", ""), // 32 hex, как md5
		strings.ToUpper(NewKey()),
		"{" + NewKey() + "}",
		"urn:uuid:" + NewKey(),
	}
	for _, n := range nonCanon {
		putAged(t, s, n, 48*time.Hour)
	}
	require.NoError(t, os.Mkdir(filepath.Join(s.Dir(), NewKey()), 0o750))

	idx := &fakeIndex{alive: map[string]struct{}{aliveOld: {}}}
	res, err := s.SweepOrphans(context.Background(), idx, time.Hour, sweepNow, quiet)
	require.NoError(t, err)

	assert.Equal(t, SweepResult{Scanned: 5, Young: 2, Removed: 2, Failed: 0}, res)
	var left []string
	for _, n := range entries(t, s.Dir()) {
		if fi, _ := os.Stat(filepath.Join(s.Dir(), n)); fi != nil && fi.Mode().IsRegular() {
			left = append(left, n)
		}
	}
	assert.Equal(t, sorted(append([]string{aliveOld, orphanYoung, ".upload-222", "readme.txt"}, nonCanon...)), sorted(left))
	assert.Equal(t, []int{2}, idx.batches, "в индекс — одна пачка из двух старых ключей; temp и молодые — нет")
}

func TestSweepOrphans_BoundaryAgeIsOrphan(t *testing.T) {
	s := newStore(t)
	k := NewKey()
	putAged(t, s, k, time.Hour) // ровно minAge — уже не «моложе»
	res, err := s.SweepOrphans(context.Background(), &fakeIndex{}, time.Hour, sweepNow, quiet)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Removed)
	assert.Empty(t, entries(t, s.Dir()))
}

func TestSweepOrphans_IndexErrorKeepsKeys(t *testing.T) {
	s := newStore(t)
	k := NewKey()
	putAged(t, s, k, 2*time.Hour)
	boom := errors.New("db down")
	res, err := s.SweepOrphans(context.Background(), &fakeIndex{err: boom}, time.Hour, sweepNow, quiet)
	require.ErrorIs(t, err, boom)
	assert.Zero(t, res.Removed)
	assert.Equal(t, []string{k}, entries(t, s.Dir()), "без ответа БД ключевой файл не удаляется")
}

func TestSweepOrphans_MassGuard(t *testing.T) {
	cases := []struct {
		name        string
		total, live int
		wantErr     bool
		wantRemoved int
	}{
		{name: "порог, ни одного живого — прерывание", total: massGuardMin, live: 0, wantErr: true},
		{name: "порог, один живой — удаление", total: massGuardMin, live: 1, wantRemoved: massGuardMin - 1},
		{name: "ниже порога, ни одного живого — удаление", total: massGuardMin - 1, live: 0, wantRemoved: massGuardMin - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newStore(t)
			alive := map[string]struct{}{}
			for i := range tc.total {
				k := NewKey()
				putAged(t, s, k, 2*time.Hour)
				if i < tc.live {
					alive[k] = struct{}{}
				}
			}
			putAged(t, s, ".upload-old", 2*time.Hour)
			res, err := s.SweepOrphans(context.Background(), &fakeIndex{alive: alive}, time.Hour, sweepNow, quiet)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrSweepSuspicious)
				assert.Len(t, entries(t, s.Dir()), tc.total, "ни один ключевой файл не удалён")
				assert.Equal(t, 1, res.Removed, "temp не зависит от БД — удалён")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRemoved+1, res.Removed)
			assert.Len(t, entries(t, s.Dir()), tc.live)
		})
	}
}

func TestIsKey(t *testing.T) {
	k := NewKey()
	assert.True(t, isKey(k))
	for _, n := range []string{"", "x", strings.ToUpper(k), strings.ReplaceAll(k, "-", ""), "{" + k + "}", "urn:uuid:" + k, k + ".tmp"} {
		assert.False(t, isKey(n), n)
	}
}

func TestSweepOrphans_Batches(t *testing.T) {
	s := newStore(t)
	const n = orphanBatch*2 + 7
	alive := map[string]struct{}{}
	for i := range n {
		k := NewKey()
		putAged(t, s, k, 2*time.Hour)
		if i%2 == 0 {
			alive[k] = struct{}{}
		}
	}
	idx := &fakeIndex{alive: alive}
	res, err := s.SweepOrphans(context.Background(), idx, time.Hour, sweepNow, quiet)
	require.NoError(t, err)
	assert.Equal(t, []int{orphanBatch, orphanBatch, 7}, idx.batches)
	assert.Equal(t, n-len(alive), res.Removed)
	assert.Len(t, entries(t, s.Dir()), len(alive))
}

func TestSweepOrphans_EmptyDirAndBadMinAge(t *testing.T) {
	s := newStore(t)
	idx := &fakeIndex{}
	res, err := s.SweepOrphans(context.Background(), idx, time.Hour, sweepNow, quiet)
	require.NoError(t, err)
	assert.Equal(t, SweepResult{}, res)
	assert.Zero(t, idx.calls(), "пустой каталог — без запроса в БД")

	_, err = s.SweepOrphans(context.Background(), idx, 0, sweepNow, quiet)
	require.Error(t, err)
}

func TestSweepOrphans_CanceledContext(t *testing.T) {
	s := newStore(t)
	k := NewKey()
	putAged(t, s, k, 2*time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.SweepOrphans(ctx, &fakeIndex{}, time.Hour, sweepNow, quiet)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{k}, entries(t, s.Dir()))
}

// Put пишет temp с тем же префиксом, что распознаёт сборщик (контракт двух половин):
// недописанный поток виден в каталоге именно как tempPrefix*.
func TestPut_TempPrefixMatchesSweeper(t *testing.T) {
	s := newStore(t)
	pr, pw := io.Pipe()
	errc := make(chan error, 1)
	go func() {
		_, err := s.Put(context.Background(), NewKey(), pr, 1<<20)
		errc <- err
	}()
	_, err := pw.Write([]byte("partial"))
	require.NoError(t, err)
	names := entries(t, s.Dir())
	require.Len(t, names, 1)
	assert.True(t, strings.HasPrefix(names[0], tempPrefix), names[0])
	require.NoError(t, pw.Close())
	require.NoError(t, <-errc)
}

func TestRunOrphanCollector_SweepsAtStartAndStopsOnCancel(t *testing.T) {
	s := newStore(t)
	k := NewKey()
	p := filepath.Join(s.Dir(), k)
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(p, old, old))

	idx := &fakeIndex{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RunOrphanCollector(ctx, s, idx, 10*time.Millisecond, time.Hour, quiet)
		close(done)
	}()

	require.Eventually(t, func() bool { return len(entries(t, s.Dir())) == 0 }, 2*time.Second, 5*time.Millisecond)
	// следующий файл-сирота подбирается уже тиком, не стартовым проходом
	k2 := NewKey()
	p2 := filepath.Join(s.Dir(), k2)
	require.NoError(t, os.WriteFile(p2, []byte("x"), 0o600))
	require.NoError(t, os.Chtimes(p2, old, old))
	require.Eventually(t, func() bool { return len(entries(t, s.Dir())) == 0 }, 2*time.Second, 5*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("сборщик не остановился по отмене ctx")
	}
}
