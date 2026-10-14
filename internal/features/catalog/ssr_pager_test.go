package catalog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseSSROffset — pagination DS-005 (к TS-005/TS-006): невалидный/отрицательный → 0,
// некратный 100 — вниз до начала страницы.
func TestParseSSROffset(t *testing.T) {
	cases := map[string]int{"": 0, "abc": 0, "-5": 0, "0": 0, "100": 100, "150": 100, "2300": 2300, "99": 0}
	for raw, want := range cases {
		require.Equal(t, want, parseSSROffset(raw), "raw=%q", raw)
	}
}

// TestBuildPager — pagination DS-005: подпись «Показаны 101–200 из 2345», ссылки сохраняют
// q/category/archived; за пределами total — «Назад» на последнюю непустую страницу.
func TestBuildPager(t *testing.T) {
	p := buildPager("/catalog/products", "болт", "c1", true, 100, 100, 2345)
	require.True(t, p.Show)
	require.Equal(t, 101, p.From)
	require.Equal(t, 200, p.To)
	require.Equal(t, "/catalog/products?archived=1&category=c1&q=%D0%B1%D0%BE%D0%BB%D1%82", p.PrevURL)
	require.Equal(t, "/catalog/products?archived=1&category=c1&offset=200&q=%D0%B1%D0%BE%D0%BB%D1%82", p.NextURL)

	p = buildPager("/catalog/products", "", "", false, 0, 50, 50)
	require.False(t, p.Show, "одна страница — пейджер не нужен")
	require.Empty(t, p.PrevURL)
	require.Empty(t, p.NextURL)

	p = buildPager("/catalog/services", "", "", false, 2300, 45, 2345)
	require.Equal(t, 2301, p.From)
	require.Equal(t, 2345, p.To)
	require.Empty(t, p.NextURL)
	require.Equal(t, "/catalog/services?offset=2200", p.PrevURL)

	p = buildPager("/catalog/services", "", "", false, 5000, 0, 250)
	require.True(t, p.Show)
	require.Zero(t, p.From)
	require.Empty(t, p.NextURL)
	require.Equal(t, "/catalog/services?offset=200", p.PrevURL)

	p = buildPager("/catalog/services", "", "", false, 100, 0, 0)
	require.Equal(t, "/catalog/services", p.PrevURL)
}
