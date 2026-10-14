package config

import (
	"fmt"
	"os"
	"time"
)

// Config — env-контракт приложения. Имена переменных — референс LiveSuite (§TD-8):
// DATABASE_URL, REDIS_ADDR, PORT. Auth-часть — internal/config/auth.go (штампуется
// фрагментом project-bootstrap под выбранный auth.mode).
type Config struct {
	DatabaseURL  string
	RedisAddr    string
	Port         string
	LogLevel     string
	OTLPEndpoint string // пусто → трейсинг no-op
	UploadsDir   string // каталог LocalFileStore вложений заказов (фаза 7), дефолт ./uploads
	UploadsGC    UploadsGC
	Auth         AuthConfig
	Firm         FirmInfo
	// TestPurgeEnabled — служебная физическая уборка тестовых данных автотестами
	// (DELETE /api/v1/test/purge/:kind/:id, soft-delete-core DS-027). Только
	// TEST_PURGE_ENABLED=true; по умолчанию и на рабочих стендах выключено — адреса нет.
	TestPurgeEnabled bool
}

// FirmInfo — реквизиты фирмы для печатных актов (SSR-печать, ADR-102). Все поля —
// пустые строки допустимы (нет обязательности): пустой блок реквизитов в шаблоне
// печати опускается по каждому полю отдельно, старт приложения не падает.
type FirmInfo struct {
	Name    string
	Address string
	Phone   string
}

// UploadsGC — фоновый сборщик файлов-сирот UPLOADS_DIR (файл без строки task_attachments
// после падения процесса между записью файла и commit). Interval = 0 — сборщик выключен.
type UploadsGC struct {
	Interval time.Duration // период проходов; первый — сразу при старте
	MinAge   time.Duration // файл моложе не трогается (идущая загрузка)
}

// uploadsGCMinAgeFloor — нижняя граница MinAge: меньшее значение рискует удалить файл
// загрузки, строка которой ещё не вставлена.
const uploadsGCMinAgeFloor = time.Minute

// uploadsGCIntervalFloor — нижняя граница периода: чаще — горячий цикл по каталогу и БД.
// Live-тест (UPLOADS_GC_INTERVAL=1s) — ровно на границе.
const uploadsGCIntervalFloor = time.Second

func loadUploadsGC() (UploadsGC, error) {
	var g UploadsGC
	raw := getenv("UPLOADS_GC_INTERVAL", "6h")
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 || (d > 0 && d < uploadsGCIntervalFloor) {
		return g, fmt.Errorf("UPLOADS_GC_INTERVAL '%s': ожидается 0 (выключить) или длительность ≥ %s", raw, uploadsGCIntervalFloor)
	}
	g.Interval = d
	if g.MinAge, err = duration("UPLOADS_GC_MIN_AGE", "1h"); err != nil {
		return g, err
	}
	if g.MinAge < uploadsGCMinAgeFloor {
		return g, fmt.Errorf("UPLOADS_GC_MIN_AGE (%s) меньше %s", g.MinAge, uploadsGCMinAgeFloor)
	}
	return g, nil
}

// loadTestPurgeEnabled — служебная уборка включается ТОЛЬКО точным значением "true"
// (soft-delete-core DS-027): пусто, "1", "TRUE", "yes" — выключено, чтобы адрес физического
// удаления не появился на рабочем стенде по неосторожности.
func loadTestPurgeEnabled() bool { return os.Getenv("TEST_PURGE_ENABLED") == "true" }

func loadFirm() FirmInfo {
	return FirmInfo{
		Name:    os.Getenv("FIRM_NAME"),
		Address: os.Getenv("FIRM_ADDRESS"),
		Phone:   os.Getenv("FIRM_PHONE"),
	}
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		RedisAddr:    getenv("REDIS_ADDR", "127.0.0.1:6379"),
		Port:         getenv("PORT", "8080"),
		LogLevel:     getenv("LOG_LEVEL", "info"),
		OTLPEndpoint: os.Getenv("OTLP_ENDPOINT"),
		UploadsDir:   getenv("UPLOADS_DIR", "./uploads"),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL обязателен")
	}
	auth, err := loadAuth()
	if err != nil {
		return nil, err
	}
	cfg.Auth = auth
	if cfg.UploadsGC, err = loadUploadsGC(); err != nil {
		return nil, err
	}
	cfg.Firm = loadFirm()
	cfg.TestPurgeEnabled = loadTestPurgeEnabled()
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
