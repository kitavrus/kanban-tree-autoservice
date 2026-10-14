// Package session — opaque-сессии в Redis (plan.md §2, D35).
//
//	session:<sid>        → JSON {user_id, created_at}, TTL = sliding TTL (продлевается на каждом Get)
//	user_sessions:<uid>  → SET sid'ов пользователя — убить все сессии при блокировке/смене пароля
//
// Абсолютный потолок: сессия старше absoluteMax убивается на Get независимо от активности.
// Роль/активность в сессии НЕ хранятся — middleware читает их из БД на каждом запросе,
// поэтому смена роли действует сразу, без перелогина.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNoSession — сессии нет: не было, истекла по TTL/потолку или убита.
var ErrNoSession = errors.New("session: no session")

const (
	sessionPrefix = "session:"
	indexPrefix   = "user_sessions:"
)

// Data — содержимое сессии.
type Data struct {
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	rdb         *redis.Client
	ttl         time.Duration
	absoluteMax time.Duration
	now         func() time.Time
}

func NewStore(rdb *redis.Client, ttl, absoluteMax time.Duration) *Store {
	return &Store{rdb: rdb, ttl: ttl, absoluteMax: absoluteMax, now: time.Now}
}

func sessionKey(sid string) string  { return sessionPrefix + sid }
func indexKey(userID string) string { return indexPrefix + userID }

// Create — новая сессия пользователя; возвращает sid (256 бит случайности).
func (s *Store) Create(ctx context.Context, userID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	sid := hex.EncodeToString(raw)
	payload, err := json.Marshal(Data{UserID: userID, CreatedAt: s.now().UTC()})
	if err != nil {
		return "", err
	}
	if err := s.pruneIndex(ctx, userID); err != nil {
		return "", err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, sessionKey(sid), payload, s.ttl)
	pipe.SAdd(ctx, indexKey(userID), sid)
	pipe.Expire(ctx, indexKey(userID), s.absoluteMax)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("session create: %w", err)
	}
	return sid, nil
}

// pruneIndex — убрать из индекса пользователя sid'ы истёкших по TTL сессий: иначе
// у часто входящего пользователя индекс растёт без предела (ключ продлевается входом).
func (s *Store) pruneIndex(ctx context.Context, userID string) error {
	sids, err := s.rdb.SMembers(ctx, indexKey(userID)).Result()
	if err != nil || len(sids) == 0 {
		return err
	}
	pipe := s.rdb.Pipeline()
	exists := make([]*redis.IntCmd, len(sids))
	for i, sid := range sids {
		exists[i] = pipe.Exists(ctx, sessionKey(sid))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("session prune: %w", err)
	}
	var dead []any
	for i, e := range exists {
		if e.Val() == 0 {
			dead = append(dead, sids[i])
		}
	}
	if len(dead) == 0 {
		return nil
	}
	return s.rdb.SRem(ctx, indexKey(userID), dead...).Err()
}

// Get — данные сессии со sliding-продлением TTL; ErrNoSession — сессии нет.
func (s *Store) Get(ctx context.Context, sid string) (*Data, error) {
	if sid == "" {
		return nil, ErrNoSession
	}
	raw, err := s.rdb.Get(ctx, sessionKey(sid)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("session get: %w", err)
	}
	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("session decode: %w", err)
	}
	if s.now().Sub(d.CreatedAt) >= s.absoluteMax {
		_ = s.Kill(ctx, sid)
		return nil, ErrNoSession
	}
	// TTL не выходит за потолок: иначе ключ переживёт absoluteMax в Redis
	ttl := s.ttl
	if left := s.absoluteMax - s.now().Sub(d.CreatedAt); left < ttl {
		ttl = left
	}
	if err := s.rdb.Expire(ctx, sessionKey(sid), ttl).Err(); err != nil {
		return nil, fmt.Errorf("session renew: %w", err)
	}
	return &d, nil
}

// Kill — удалить одну сессию (выход).
func (s *Store) Kill(ctx context.Context, sid string) error {
	raw, err := s.rdb.Get(ctx, sessionKey(sid)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("session kill: %w", err)
	}
	var d Data
	_ = json.Unmarshal(raw, &d)
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, sessionKey(sid))
	if d.UserID != "" {
		pipe.SRem(ctx, indexKey(d.UserID), sid)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// KillUser — удалить все сессии пользователя, кроме exceptSID (пусто — все).
// Возвращает число убитых сессий.
func (s *Store) KillUser(ctx context.Context, userID, exceptSID string) (int, error) {
	sids, err := s.rdb.SMembers(ctx, indexKey(userID)).Result()
	if err != nil {
		return 0, fmt.Errorf("session kill user: %w", err)
	}
	pipe := s.rdb.TxPipeline()
	killed := 0
	for _, sid := range sids {
		if sid == exceptSID {
			continue
		}
		pipe.Del(ctx, sessionKey(sid))
		pipe.SRem(ctx, indexKey(userID), sid)
		killed++
	}
	if killed == 0 {
		return 0, nil
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("session kill user: %w", err)
	}
	return killed, nil
}
