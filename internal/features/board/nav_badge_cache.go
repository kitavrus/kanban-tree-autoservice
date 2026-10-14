package board

import (
	"context"
	"sync"
	"time"
)

// navBadgeTTL — сколько SSR-бейдж «Остатки» живёт в кэше. Кэш снимает тяжёлый
// stockPendingCTE с каждого SSR-рендера. Любой успешный изменяющий запрос сбрасывает кэш
// (StockService.InvalidateNavBadges из middleware в router.go), так что после правки
// следующая страница показывает свежее число. TTL страхует записи в БД мимо приложения
// (seed-demo, соседний инстанс при blue-green); их, как и раньше, сразу поправляет
// nav-badge.js — он после загрузки страницы берёт risk_count из некэшируемого API.
const navBadgeTTL = 10 * time.Second

// navBadgeCache — одно число с TTL. Промах считает значение под мьютексом: параллельные
// рендеры после истечения TTL ждут один запрос, а не запускают по своему. Ошибка
// загрузки не кэшируется.
type navBadgeCache struct {
	ttl  time.Duration
	now  func() time.Time
	load func(context.Context) (int, error)

	mu  sync.Mutex
	at  time.Time
	val int
	ok  bool
}

func newNavBadgeCache(ttl time.Duration, load func(context.Context) (int, error)) *navBadgeCache {
	return &navBadgeCache{ttl: ttl, now: time.Now, load: load}
}

// invalidate — следующий get пойдёт в БД.
func (c *navBadgeCache) invalidate() {
	c.mu.Lock()
	c.ok = false
	c.mu.Unlock()
}

func (c *navBadgeCache) get(ctx context.Context) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ok && c.now().Sub(c.at) < c.ttl {
		return c.val, nil
	}
	v, err := c.load(ctx)
	if err != nil {
		return 0, err
	}
	c.val, c.at, c.ok = v, c.now(), true
	return v, nil
}
