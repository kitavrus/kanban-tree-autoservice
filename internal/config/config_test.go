package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUploadsGC_Defaults(t *testing.T) {
	t.Setenv("UPLOADS_GC_INTERVAL", "")
	t.Setenv("UPLOADS_GC_MIN_AGE", "")
	g, err := loadUploadsGC()
	require.NoError(t, err)
	assert.Equal(t, UploadsGC{Interval: 6 * time.Hour, MinAge: time.Hour}, g)
}

func TestLoadUploadsGC_Values(t *testing.T) {
	cases := []struct {
		name, interval, minAge string
		want                   UploadsGC
		wantErr                bool
	}{
		{name: "выключен", interval: "0", minAge: "1h", want: UploadsGC{Interval: 0, MinAge: time.Hour}},
		{name: "свои значения", interval: "30m", minAge: "2h", want: UploadsGC{Interval: 30 * time.Minute, MinAge: 2 * time.Hour}},
		{name: "граница min age", interval: "1h", minAge: "1m", want: UploadsGC{Interval: time.Hour, MinAge: time.Minute}},
		{name: "min age ниже порога", interval: "1h", minAge: "30s", wantErr: true},
		{name: "min age ноль", interval: "1h", minAge: "0", wantErr: true},
		{name: "граница интервала", interval: "1s", minAge: "1h", want: UploadsGC{Interval: time.Second, MinAge: time.Hour}},
		{name: "интервал ниже порога", interval: "500ms", minAge: "1h", wantErr: true},
		{name: "отрицательный интервал", interval: "-1h", minAge: "1h", wantErr: true},
		{name: "мусор в интервале", interval: "often", minAge: "1h", wantErr: true},
		{name: "мусор в min age", interval: "1h", minAge: "old", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("UPLOADS_GC_INTERVAL", tc.interval)
			t.Setenv("UPLOADS_GC_MIN_AGE", tc.minAge)
			g, err := loadUploadsGC()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, g)
		})
	}
}

func TestLoad_UploadsGCErrorFailsConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("UPLOADS_GC_MIN_AGE", "5s")
	_, err := Load()
	require.ErrorContains(t, err, "UPLOADS_GC_MIN_AGE")
}

// soft-delete-core DS-027: служебная уборка тестовых данных выключена по умолчанию и
// включается только точным TEST_PURGE_ENABLED=true.
func TestLoadTestPurgeEnabled(t *testing.T) {
	for value, want := range map[string]bool{"": false, "false": false, "0": false, "1": false, "TRUE": false, "yes": false, " true": false, "true": true} {
		t.Setenv("TEST_PURGE_ENABLED", value)
		assert.Equal(t, want, loadTestPurgeEnabled(), "TEST_PURGE_ENABLED=%q", value)
	}
}
