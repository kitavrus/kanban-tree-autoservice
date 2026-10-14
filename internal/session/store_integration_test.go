//go:build integration

package session

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// StoreSuite — хранилище сессий против настоящего Redis (dockertest, BaseSuite).
type StoreSuite struct {
	testutil.BaseSuite
	rdb *redis.Client
	ctx context.Context
}

func TestStore(t *testing.T) { suite.Run(t, new(StoreSuite)) }

func (s *StoreSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()
	s.rdb = redis.NewClient(&redis.Options{Addr: s.RedisAddr()})
	s.ctx = context.Background()
}

func (s *StoreSuite) TearDownSuite() {
	_ = s.rdb.Close()
	s.BaseSuite.TearDownSuite()
}

// clock — управляемое время для проверки потолка.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func (s *StoreSuite) newStore(ttl, max time.Duration) (*Store, *clock) {
	c := &clock{t: time.Now()}
	st := NewStore(s.rdb, ttl, max)
	st.now = c.now
	return st, c
}

func (s *StoreSuite) TestCreateGet() {
	st, _ := s.newStore(time.Hour, 24*time.Hour)
	sid, err := st.Create(s.ctx, "u-create")
	s.Require().NoError(err)
	s.Len(sid, 64)

	d, err := st.Get(s.ctx, sid)
	s.Require().NoError(err)
	s.Equal("u-create", d.UserID)

	_, err = st.Get(s.ctx, "nope")
	s.ErrorIs(err, ErrNoSession)
	_, err = st.Get(s.ctx, "")
	s.ErrorIs(err, ErrNoSession)
}

func (s *StoreSuite) TestSlidingRenewsTTL() {
	st, _ := s.newStore(time.Hour, 24*time.Hour)
	sid, err := st.Create(s.ctx, "u-sliding")
	s.Require().NoError(err)
	s.Require().NoError(s.rdb.Expire(s.ctx, sessionKey(sid), time.Minute).Err())

	_, err = st.Get(s.ctx, sid)
	s.Require().NoError(err)
	ttl := s.rdb.TTL(s.ctx, sessionKey(sid)).Val()
	s.Greater(ttl, 59*time.Minute, "Get продлевает TTL до полного")
}

func (s *StoreSuite) TestTTLExpiry() {
	st, _ := s.newStore(time.Second, time.Hour)
	sid, err := st.Create(s.ctx, "u-ttl")
	s.Require().NoError(err)
	// ждём истечения, НЕ вызывая Get: он продлевает TTL (sliding) и сессия не истечёт
	s.Require().Eventually(func() bool {
		return s.rdb.Exists(s.ctx, sessionKey(sid)).Val() == 0
	}, 5*time.Second, 100*time.Millisecond)
	_, err = st.Get(s.ctx, sid)
	s.ErrorIs(err, ErrNoSession)
}

func (s *StoreSuite) TestAbsoluteMax() {
	st, c := s.newStore(time.Hour, 2*time.Hour)
	sid, err := st.Create(s.ctx, "u-abs")
	s.Require().NoError(err)

	c.t = c.t.Add(90 * time.Minute)
	_, err = st.Get(s.ctx, sid)
	s.Require().NoError(err, "активная сессия живёт до потолка")
	ttl := s.rdb.TTL(s.ctx, sessionKey(sid)).Val()
	s.LessOrEqual(ttl, 31*time.Minute, "продление не выходит за потолок")

	c.t = c.t.Add(31 * time.Minute)
	_, err = st.Get(s.ctx, sid)
	s.ErrorIs(err, ErrNoSession, "после потолка — перелогин даже при активности")
	s.Zero(s.rdb.Exists(s.ctx, sessionKey(sid)).Val(), "ключ удалён")
}

func (s *StoreSuite) TestKill() {
	st, _ := s.newStore(time.Hour, 24*time.Hour)
	sid, err := st.Create(s.ctx, "u-kill")
	s.Require().NoError(err)
	s.Require().NoError(st.Kill(s.ctx, sid))
	_, err = st.Get(s.ctx, sid)
	s.ErrorIs(err, ErrNoSession)
	s.False(s.rdb.SIsMember(s.ctx, indexKey("u-kill"), sid).Val())
	s.NoError(st.Kill(s.ctx, sid), "повторный Kill — не ошибка")
}

func (s *StoreSuite) TestKillUser() {
	st, _ := s.newStore(time.Hour, 24*time.Hour)
	a, _ := st.Create(s.ctx, "u-all")
	b, _ := st.Create(s.ctx, "u-all")
	c, _ := st.Create(s.ctx, "u-all")
	other, _ := st.Create(s.ctx, "u-other")

	n, err := st.KillUser(s.ctx, "u-all", b)
	s.Require().NoError(err)
	s.Equal(2, n)
	_, err = st.Get(s.ctx, a)
	s.ErrorIs(err, ErrNoSession)
	_, err = st.Get(s.ctx, c)
	s.ErrorIs(err, ErrNoSession)
	_, err = st.Get(s.ctx, b)
	s.NoError(err, "исключённая сессия жива")
	_, err = st.Get(s.ctx, other)
	s.NoError(err, "чужие сессии не тронуты")

	n, err = st.KillUser(s.ctx, "u-all", "")
	s.Require().NoError(err)
	s.Equal(1, n)
	n, err = st.KillUser(s.ctx, "u-nobody", "")
	s.Require().NoError(err)
	s.Zero(n)
}

func (s *StoreSuite) TestCreatePrunesDeadSessionsFromIndex() {
	st, _ := s.newStore(time.Hour, 24*time.Hour)
	dead, _ := st.Create(s.ctx, "u-prune")
	alive, _ := st.Create(s.ctx, "u-prune")
	s.Require().NoError(s.rdb.Del(s.ctx, sessionKey(dead)).Err()) // истекла по TTL

	fresh, err := st.Create(s.ctx, "u-prune")
	s.Require().NoError(err)
	members := s.rdb.SMembers(s.ctx, indexKey("u-prune")).Val()
	s.ElementsMatch([]string{alive, fresh}, members, "мёртвый sid вычищен из индекса")
}
