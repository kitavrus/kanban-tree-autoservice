//go:build integration

package testutil

import (
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// LiveSuite (CLAUDE.md §TD-8): боевой бинарь + реальная сеть поверх BaseSuite.
// Сьюта обязана задать BinaryPkg до SetupSuite. Auth-стабы запрещены — LoginAs
// (login_*.go, штампуется под auth.mode) ходит в реальный endpoint.
type LiveSuite struct {
	BaseSuite

	BinaryPkg   string   // пакет бинаря: "./cmd/server"
	ExtraEnv    []string // доп. env поверх обязательных
	HealthzPath string   // дефолт "/health"

	BaseURL string
	Client  *http.Client

	cmd    *exec.Cmd
	exited chan struct{} // закрывается, когда subprocess завершился
}

// moduleRoot — корень Go-модуля (каталог с go.mod) от текущего файла.
func moduleRoot() string {
	_, f, _, _ := runtime.Caller(0)
	d := filepath.Dir(f)
	for d != "/" {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	return "."
}

// Порт сервера выбирается ВНЕ эфемерного диапазона ОС (49152+ на macOS, 32768+ на Linux),
// из которого docker раздаёт host-порты контейнерам: сервер слушает wildcard (":PORT"), а
// контейнер на 127.0.0.1:<тот же порт> биндится без ошибки и перехватывает loopback-трафик.
const (
	serverPortMin   = 20000
	serverPortSpan  = 10000
	serverAttempts  = 5
	serverReadyWait = 30 * time.Second
)

// freePort — случайный порт из [serverPortMin, serverPortMin+serverPortSpan), свободный и
// на wildcard (его займёт сервер), и на loopback (по нему ходит клиент).
func freePort() (int, error) {
	var lastErr error
	for i := 0; i < 50; i++ {
		port := serverPortMin + rand.IntN(serverPortSpan)
		if lastErr = probePort(port); lastErr == nil {
			return port, nil
		}
	}
	return 0, fmt.Errorf("нет свободного порта в %d–%d: %w", serverPortMin, serverPortMin+serverPortSpan-1, lastErr)
}

func probePort(port int) error {
	for _, host := range []string{"0.0.0.0", "127.0.0.1"} {
		ln, err := net.Listen("tcp4", fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return err
		}
		_ = ln.Close()
	}
	return nil
}

func (s *LiveSuite) SetupSuite() {
	s.Require().NotEmpty(s.BinaryPkg, "LiveSuite.BinaryPkg обязателен (§TD-8)")
	s.BaseSuite.SetupSuite() // dockertest PG+Redis + миграции

	// Require ниже = FailNow: testify TearDownSuite уже не вызовет — убираем сами.
	ready := false
	defer func() {
		if !ready {
			s.TearDownSuite()
		}
	}()

	// 1. Боевой бинарь — go build (одна сборка на сьюту).
	binPath := filepath.Join(s.T().TempDir(), "server")
	build := exec.Command("go", "build", "-o", binPath, s.BinaryPkg)
	build.Dir = moduleRoot()
	if out, err := build.CombinedOutput(); err != nil {
		s.Require().NoError(err, "go build %s:\n%s", s.BinaryPkg, out)
	}

	jar, _ := cookiejar.New(nil)
	s.Client = &http.Client{Jar: jar, Timeout: 10 * time.Second}

	// 2. Запуск subprocess'ом на тест-контейнеры, случайный порт. Между freePort и bind
	// сервера порт может занять параллельный пакет — тогда бинарь выходит (listen error),
	// и старт повторяется на другом порту.
	for attempt := 1; ; attempt++ {
		err := s.startServer(binPath)
		if err == nil {
			break
		}
		s.stopServer()
		s.Require().Less(attempt, serverAttempts, "бинарь не поднялся за %d попыток: %v", serverAttempts, err)
		s.T().Logf("testutil: попытка %d — %v; перезапускаю на другом порту", attempt, err)
	}
	ready = true
}

// startServer запускает бинарь и ждёт /health == 200 (детерминированный retry, не
// sleep-вслепую). Ответ засчитывается, только пока жив НАШ процесс.
func (s *LiveSuite) startServer(binPath string) error {
	port, err := freePort()
	if err != nil {
		return err
	}
	s.BaseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	cmd := exec.Command(binPath)
	cmd.Dir = moduleRoot()
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+s.DSN,
		"REDIS_ADDR="+s.RedisAddr(),
		fmt.Sprintf("PORT=%d", port),
	)
	cmd.Env = append(cmd.Env, defaultExtraEnv()...) // auth-режимный env (login.go)
	cmd.Env = append(cmd.Env, s.ExtraEnv...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	s.cmd, s.exited = cmd, exited

	hp := s.HealthzPath
	if hp == "" {
		hp = "/health"
	}
	deadline := time.Now().Add(serverReadyWait)
	for {
		resp, err := s.Client.Get(s.BaseURL + hp)
		healthy := false
		if err == nil {
			resp.Body.Close()
			healthy = resp.StatusCode == http.StatusOK
		}
		select {
		case <-exited:
			return fmt.Errorf("бинарь завершился при старте (порт %d): %v", port, cmd.ProcessState)
		default:
		}
		if healthy {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("бинарь не ответил на %s за %s (порт %d): %v", hp, serverReadyWait, port, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *LiveSuite) stopServer() {
	if s.cmd == nil {
		return
	}
	_ = s.cmd.Process.Kill()
	<-s.exited
	s.cmd = nil
}

func (s *LiveSuite) TearDownSuite() {
	s.stopServer()
	s.BaseSuite.TearDownSuite()
}
