//go:build integration

// Package testutil — интеграционная база (§TD-8): dockertest PG+Redis + миграции.
package testutil

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	mobyclient "github.com/moby/moby/client"
	"github.com/ory/dockertest/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

// Метки тест-контейнеров. По labelRun прогон убирает ТОЛЬКО свои контейнеры (trap в
// Makefile), по labelHost+labelPID сьюта находит брошенные (владелец умер без TearDown).
const (
	labelRun  = "kanban-tree.testutil.run"
	labelPID  = "kanban-tree.testutil.pid"
	labelHost = "kanban-tree.testutil.host"

	// RunIDEnv — идентификатор прогона (ставит make test-integration*); без него — pid процесса.
	RunIDEnv = "TESTUTIL_RUN_ID"

	infraAttempts    = 3                // пересоздание пары PG+Redis при неготовности
	infraAttemptWait = 60 * time.Second // ожидание готовности на одну попытку
)

// BaseSuite — тест-контейнеры PG18 + Redis, миграции применены. Экспортирует
// Pool/DSN/RedisAddr() — их использует LiveSuite и repository-интеграция.
type BaseSuite struct {
	suite.Suite

	Pool *pgxpool.Pool
	DSN  string

	redisAddr string
	pool      dockertest.ClosablePool
	pgRes     dockertest.ClosableResource
	redisRes  dockertest.ClosableResource
}

func (s *BaseSuite) RedisAddr() string { return s.redisAddr }

func (s *BaseSuite) SetupSuite() {
	// Require в SetupSuite = FailNow: testify TearDownSuite уже не вызовет — убираем сами.
	ready := false
	defer func() {
		if !ready {
			s.purgeInfra()
			if s.pool != nil {
				_ = s.pool.Close(context.Background())
			}
		}
	}()

	var err error
	s.pool, err = dockertest.NewPool(context.Background(), "")
	s.Require().NoError(err, "docker недоступен")
	s.reapOrphans()

	for attempt := 1; ; attempt++ {
		if err = s.startInfra(); err == nil {
			break
		}
		s.purgeInfra()
		s.Require().Less(attempt, infraAttempts, "PG/Redis не поднялись за %d попыток: %v", infraAttempts, err)
		s.T().Logf("testutil: попытка %d — %v; пересоздаю контейнеры", attempt, err)
	}

	mig, err := migrate.New("file://"+moduleRoot()+"/migrations", s.DSN)
	s.Require().NoError(err)
	s.Require().NoError(mig.Up())
	srcErr, dbErr := mig.Close()
	s.Require().NoError(srcErr)
	s.Require().NoError(dbErr)

	s.Pool, err = pgxpool.New(context.Background(), s.DSN)
	s.Require().NoError(err)
	ready = true
}

// startInfra поднимает пару PG+Redis и ждёт, пока по их портам ответят именно PG и Redis.
func (s *BaseSuite) startInfra() error {
	var err error
	s.pgRes, err = s.runContainer("postgres", "18", "5432/tcp",
		"POSTGRES_USER=test", "POSTGRES_PASSWORD=test", "POSTGRES_DB=test")
	if err != nil {
		return fmt.Errorf("запуск postgres: %w", err)
	}
	s.DSN = fmt.Sprintf("postgres://test:test@127.0.0.1:%s/test?sslmode=disable",
		s.pgRes.GetPort("5432/tcp"))

	s.redisRes, err = s.runContainer("redis", "8", "6379/tcp")
	if err != nil {
		return fmt.Errorf("запуск redis: %w", err)
	}
	s.redisAddr = "127.0.0.1:" + s.redisRes.GetPort("6379/tcp")

	deadline := time.Now().Add(infraAttemptWait)
	if err := s.waitReady(s.pgRes, deadline, func(ctx context.Context) error {
		p, err := pgxpool.New(ctx, s.DSN)
		if err != nil {
			return err
		}
		defer p.Close()
		return p.Ping(ctx)
	}); err != nil {
		return fmt.Errorf("postgres (%s): %w", s.DSN, err)
	}
	if err := s.waitReady(s.redisRes, deadline, func(ctx context.Context) error {
		c := redis.NewClient(&redis.Options{Addr: s.redisAddr})
		defer c.Close()
		return c.Ping(ctx).Err()
	}); err != nil {
		return fmt.Errorf("redis (%s): %w", s.redisAddr, err)
	}
	return nil
}

// runContainer публикует порт ТОЛЬКО на 127.0.0.1. При публикации на 0.0.0.0 (дефолт
// dockertest) любой сокет хоста на 127.0.0.1:<тот же порт> — контейнер соседнего
// compose-стенда, чужой сервер — молча перехватывает трафик: bind проходит у обоих.
// WithoutReuse: v4 по умолчанию переиспользует контейнер по repo:tag — каждой сьюте нужна
// своя чистая пара PG+Redis.
func (s *BaseSuite) runContainer(repo, tag, port string, env ...string) (dockertest.ClosableResource, error) {
	host, _ := os.Hostname()
	p, err := network.ParsePort(port)
	if err != nil {
		return nil, err
	}
	res, err := s.pool.Run(context.Background(), repo,
		dockertest.WithTag(tag),
		dockertest.WithEnv(env),
		dockertest.WithoutReuse(),
		dockertest.WithLabels(map[string]string{
			labelRun:  runID(),
			labelPID:  strconv.Itoa(os.Getpid()),
			labelHost: host,
		}),
		dockertest.WithPortBindings(network.PortMap{
			p: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: ""}},
		}),
		dockertest.WithHostConfig(func(hc *container.HostConfig) { hc.PublishAllPorts = false }),
	)
	if err != nil {
		return nil, err
	}
	if res.GetPort(port) == "" || res.GetBoundIP(port) != "127.0.0.1" {
		_ = res.Close(context.Background())
		return nil, fmt.Errorf("%s: порт %s опубликован не на 127.0.0.1 (%q)", repo, port, res.GetHostPort(port))
	}
	return res, nil
}

// waitReady опрашивает check до deadline; контейнер умер — выходит сразу, не дожидаясь.
func (s *BaseSuite) waitReady(res dockertest.ClosableResource, deadline time.Time, check func(context.Context) error) error {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := check(ctx)
		cancel()
		if err == nil {
			return nil
		}
		insp, inspErr := s.pool.Client().ContainerInspect(context.Background(), res.ID(), mobyclient.ContainerInspectOptions{})
		if inspErr != nil {
			return fmt.Errorf("контейнер пропал (%v); последняя ошибка: %w", inspErr, err)
		}
		if st := insp.Container.State; st == nil || !st.Running {
			return fmt.Errorf("контейнер завершился (%s); последняя ошибка: %w", containerState(st), err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("не готов за %s: %w", infraAttemptWait, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *BaseSuite) purgeInfra() {
	if s.pool == nil {
		return
	}
	for _, r := range []*dockertest.ClosableResource{&s.pgRes, &s.redisRes} {
		if *r != nil {
			_ = (*r).Close(context.Background())
			*r = nil
		}
	}
}

// reapOrphans удаляет контейнеры testutil, чей процесс-владелец на этой машине уже мёртв
// (go test -timeout, kill -9 — TearDownSuite не отработал). Контейнеры живых процессов —
// параллельные пакеты, соседняя сессия/ворктри — не трогаются.
func (s *BaseSuite) reapOrphans() {
	host, _ := os.Hostname()
	ctx := context.Background()
	list, err := s.pool.Client().ContainerList(ctx, mobyclient.ContainerListOptions{
		All:     true,
		Filters: mobyclient.Filters{}.Add("label", labelHost+"="+host),
	})
	if err != nil {
		return
	}
	for _, c := range list.Items {
		pid, err := strconv.Atoi(c.Labels[labelPID])
		if err != nil || pid <= 0 || !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			continue
		}
		_, _ = s.pool.Client().ContainerRemove(ctx, c.ID, mobyclient.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	}
}

func runID() string {
	if id := os.Getenv(RunIDEnv); id != "" {
		return id
	}
	return "pid-" + strconv.Itoa(os.Getpid())
}

func (s *BaseSuite) TearDownSuite() {
	if s.Pool != nil {
		s.Pool.Close()
	}
	s.purgeInfra()
	if s.pool != nil {
		_ = s.pool.Close(context.Background())
	}
}

func containerState(st *container.State) string {
	if st == nil {
		return "нет состояния"
	}
	return fmt.Sprintf("%s, exit=%d", st.Status, st.ExitCode)
}
