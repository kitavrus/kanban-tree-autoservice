//go:build integration

package suppliers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление поставщиков (docs/features/soft-delete-suppliers/draft-spec.md §4.2, §6.3,
// §6.4, §6.5, §11) — методы SuppliersLiveSuite (§TD-8: боевой бинарь, реальный логин): TS-032
// (удаление, название свободно, NAME_TAKEN при восстановлении), TS-033 (права), причина
// удаления, служебная уборка и гонки §3.

// ---- wire-типы (§TD-6.2) ----

type sdDeleteDTO struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type sdRestoreDTO struct {
	ID string `json:"id"`
}

type sdErrDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---- помощники ----

func sdMark() string { return uuid.NewString()[:8] }

func (s *SuppliersLiveSuite) sdErr(raw []byte) sdErrDTO {
	var e sdErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

func (s *SuppliersLiveSuite) sdScalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *SuppliersLiveSuite) sdAdminID() string {
	return s.sdScalar(`SELECT id::text FROM users WHERE login = 'sup-admin'`)
}

// supState — «удалён|is_active|причина|кто удалил» строки поставщика; "" — строки нет.
func (s *SuppliersLiveSuite) supState(id string) string {
	return s.sdScalar(`
		SELECT coalesce((SELECT (deleted_at IS NOT NULL)::text || '|' || is_active::text || '|' ||
		                        coalesce(delete_reason, '') || '|' || coalesce(deleted_by::text, '')
		                 FROM suppliers WHERE id = $1), '')`, id)
}

func (s *SuppliersLiveSuite) deleteSup(b *testutil.Browser, id string, reason any) (int, sdDeleteDTO, []byte) {
	var body any
	if reason != nil {
		body = map[string]any{"reason": reason}
	}
	st, raw := b.Do(http.MethodDelete, supAPI+"/"+id, body)
	var d sdDeleteDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	}
	return st, d, raw
}

func (s *SuppliersLiveSuite) mustDeleteSup(id string) {
	st, _, raw := s.deleteSup(s.admin, id, "Создан по ошибке")
	s.Require().Equal(http.StatusOK, st, "удаление поставщика %s: %s", id, raw)
}

func (s *SuppliersLiveSuite) restoreSup(b *testutil.Browser, id string) (int, []byte) {
	return b.Do(http.MethodPost, supAPI+"/"+id+"/restore", nil)
}

// listedSup — поставщик виден в списке при фильтре архива isActive ("" — все).
func (s *SuppliersLiveSuite) listedSup(id, isActive string) bool {
	q := url.Values{"limit": {"500"}}
	if isActive != "" {
		q.Set("is_active", isActive)
	}
	for _, it := range s.list(q).Items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func (s *SuppliersLiveSuite) sdExecutor(login string) *testutil.Browser {
	s.SeedUser(login, "password1", "Поставщики Исполнитель", "executor", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b
}

// ---- TS-032 ----

// TS-032 (DS-014, DS-026): поставщик удаляется — 200 {id, deleted_at}; отметка «кто, когда,
// почему» записана, причина обрезана, признак архива не изменился; соседний действующий
// поставщик не тронут. Для обычных адресов поставщика больше нет: список при любом фильтре
// архива, PUT, повторное удаление — 404.
func (s *SuppliersLiveSuite) TestTS032_DeleteSupplier() {
	m := sdMark()
	sup := s.create(map[string]any{"name": "TS032 Поставщик " + m, "contact_name": "Иван"})
	other := s.create(map[string]any{"name": "TS032 Действующий " + m})

	before := time.Now().Add(-time.Minute)
	st, d, raw := s.deleteSup(s.admin, sup.ID, "  Больше не работаем  ")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(sup.ID, d.ID)
	s.True(d.DeletedAt.After(before), "deleted_at — момент удаления")
	s.Equal(time.UTC, d.DeletedAt.Location(), "deleted_at — UTC")
	s.Equal("true|true|Больше не работаем|"+s.sdAdminID(), s.supState(sup.ID))

	s.True(strings.HasPrefix(s.supState(other.ID), "false|"), "действующий поставщик не тронут")

	for _, f := range []string{"", "true", "false"} {
		s.False(s.listedSup(sup.ID, f), "удалённого поставщика нет в списке (is_active=%q)", f)
	}
	s.Zero(s.list(url.Values{"q": {sup.Name}}).Total, "и поиском не находится")

	st, raw = s.admin.Do(http.MethodPut, supAPI+"/"+sup.ID, map[string]any{"name": sup.Name, "is_active": false})
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("SUPPLIER_NOT_FOUND", s.sdErr(raw).Code)
	st, _, raw = s.deleteSup(s.admin, sup.ID, "Повтор")
	s.Equal(http.StatusNotFound, st, "повтор удаления: %s", raw)
	s.Equal("SUPPLIER_NOT_FOUND", s.sdErr(raw).Code)
	s.Equal("Поставщик не найден", s.sdErr(raw).Message)
	s.Equal("true|true|Больше не работаем|"+s.sdAdminID(), s.supState(sup.ID), "отметка удаления не тронута")
}

// TS-032 (DS-017, DS-009; §6.4): название удалённого поставщика свободно — создание и
// переименование с тем же названием проходят; восстановление при занятом названии → 409
// NAME_TAKEN, поставщик остаётся удалённым; после переименования действующего — 200 {id}, отметка
// стёрта. Повтор восстановления → 409 NOT_DELETED; несуществующий →
// 404. Поставщик, удалённый из архива, возвращается в архив. Негатив (as-is): название
// действующего поставщика при создании по-прежнему занято — 400 name: duplicate.
func (s *SuppliersLiveSuite) TestTS032_NameFreeAfterDelete_Restore() {
	m := sdMark()
	name := "Поставщик имя " + m
	first := s.create(map[string]any{"name": name})
	s.mustDeleteSup(first.ID)

	twin := s.create(map[string]any{"name": "  " + strings.ToUpper(name) + " "}) // название свободно (без учёта регистра)
	st, raw := s.admin.Do(http.MethodPost, supAPI, map[string]any{"name": name})
	s.Equal(http.StatusBadRequest, st, "название действующего занято: %s", raw)
	s.Equal(map[string]string{"name": "duplicate"}, s.fields(raw))

	st, raw = s.restoreSup(s.admin, first.ID)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("NAME_TAKEN", s.sdErr(raw).Code)
	s.Equal("Название занято — переименуйте действующую запись", s.sdErr(raw).Message)
	s.True(strings.HasPrefix(s.supState(first.ID), "true|"), "поставщик остался удалённым")

	s.update(twin.ID, map[string]any{"name": name + " второй", "is_active": true})
	st, raw = s.restoreSup(s.admin, first.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var r sdRestoreDTO
	s.Require().NoError(json.Unmarshal(raw, &r), string(raw))
	s.Equal(first.ID, r.ID)
	s.Equal("false|true||", s.supState(first.ID), "отметка стёрта, архив прежний")
	s.True(s.listedSup(first.ID, "true"))

	st, raw = s.restoreSup(s.admin, first.ID)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("NOT_DELETED", s.sdErr(raw).Code)
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		st, raw = s.restoreSup(s.admin, id)
		s.Equal(http.StatusNotFound, st, "%s: %s", id, raw)
		s.Equal("SUPPLIER_NOT_FOUND", s.sdErr(raw).Code, id)
	}

	// удалён из архива → восстановлен в архив (DS-009).
	arch := s.create(map[string]any{"name": "Поставщик архив " + m})
	s.update(arch.ID, map[string]any{"name": arch.Name, "is_active": false})
	s.mustDeleteSup(arch.ID)
	s.True(strings.HasPrefix(s.supState(arch.ID), "true|false|"), "удаление признак архива не меняет")
	st, raw = s.restoreSup(s.admin, arch.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("false|false||", s.supState(arch.ID), "поставщик вернулся в архив")
	s.True(s.listedSup(arch.ID, "false"))
}

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): исполнитель → 403 «Нет доступа» на удалении и восстановлении —
// до поиска записи (существующий, удалённый, несуществующий и битый идентификатор) и до проверки
// причины; поставщик не изменён. Негатив: без сессии → 401.
func (s *SuppliersLiveSuite) TestTS033_SoftDeleteAdminOnly() {
	m := sdMark()
	live := s.create(map[string]any{"name": "TS033 Поставщик " + m})
	gone := s.create(map[string]any{"name": "TS033 Удалённый " + m})
	s.mustDeleteSup(gone.ID)
	exec := s.sdExecutor("sup-sd-exec-" + m)
	anon := s.NewBrowser()

	for _, id := range []string{live.ID, gone.ID, "00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		for _, reason := range []any{nil, "Чужое"} {
			st, _, raw := s.deleteSup(exec, id, reason)
			s.Equal(http.StatusForbidden, st, "DELETE %s: %s", id, raw)
			s.Equal("FORBIDDEN", s.sdErr(raw).Code)
			s.Equal("Нет доступа", s.sdErr(raw).Message)
		}
		st, raw := s.restoreSup(exec, id)
		s.Equal(http.StatusForbidden, st, "restore %s: %s", id, raw)
		s.Equal("Нет доступа", s.sdErr(raw).Message)

		st, _, _ = s.deleteSup(anon, id, "Аноним")
		s.Equal(http.StatusUnauthorized, st, id)
		st, _ = s.restoreSup(anon, id)
		s.Equal(http.StatusUnauthorized, st, id)
	}
	s.True(strings.HasPrefix(s.supState(live.ID), "false|"), "поставщик исполнителем не удалён")
	s.True(strings.HasPrefix(s.supState(gone.ID), "true|"), "поставщик исполнителем не восстановлен")
}

// ---- причина удаления ----

// DS-020 (abuse case 5): причина обязательна — нет тела, пусто, пробелы → 400 reason: required;
// 501 символ → too_long; NUL → invalid_text; поставщик не удалён. 500 символов — допустимо.
func (s *SuppliersLiveSuite) TestDeleteReasonValidation() {
	sup := s.create(map[string]any{"name": "Причина " + sdMark()})
	for name, tc := range map[string]struct {
		reason any
		want   string
	}{
		"нет тела": {nil, "required"}, "пробелы": {"   ", "required"}, "пусто": {"", "required"},
		"501 символ": {strings.Repeat("я", 501), "too_long"}, "NUL": {"при\u0000чина", "invalid_text"},
	} {
		st, _, raw := s.deleteSup(s.admin, sup.ID, tc.reason)
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Equal(map[string]string{"reason": tc.want}, s.fields(raw), name)
	}
	s.True(strings.HasPrefix(s.supState(sup.ID), "false|"), "поставщик не удалён")
	st, _, raw := s.deleteSup(s.admin, sup.ID, strings.Repeat("я", 500))
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// ---- служебная уборка ----

// DS-027 (§6.5): DELETE /api/v1/test/purge/suppliers/:id (TEST_PURGE_ENABLED=true) — действующий и
// мягко удалённый поставщик стираются физически; нет поставщика либо битый идентификатор → 404
// NOT_FOUND. Негатив: исполнитель → 403.
func (s *SuppliersLiveSuite) TestPurge_Suppliers() {
	m := sdMark()
	purge := func(b *testutil.Browser, id string) (int, []byte) {
		return b.Do(http.MethodDelete, "/api/v1/test/purge/suppliers/"+id, nil)
	}
	live := s.create(map[string]any{"name": "Purge Свободный " + m})
	gone := s.create(map[string]any{"name": "Purge Удалённый " + m})
	s.mustDeleteSup(gone.ID)

	st, raw := purge(s.sdExecutor("sup-purge-exec-"+m), live.ID)
	s.Equal(http.StatusForbidden, st, string(raw))
	s.NotEmpty(s.supState(live.ID), "исполнитель уборку не выполнил")
	for _, id := range []string{live.ID, gone.ID} {
		st, raw = purge(s.admin, id)
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.Empty(s.supState(id), "строка поставщика физически удалена")
		st, raw = purge(s.admin, id)
		s.Equal(http.StatusNotFound, st, string(raw))
		s.Equal("NOT_FOUND", s.sdErr(raw).Code)
	}
	st, raw = purge(s.admin, "not-uuid")
	s.Equal(http.StatusNotFound, st, string(raw))
}

// ---- гонки (§3 «Конкурентность») ----

func (s *SuppliersLiveSuite) lockWaiters() int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND pid <> pg_backend_pid()`).Scan(&n))
	return n
}

type raceResult struct {
	status int
	raw    []byte
}

// race — запрос req уходит, пока тест держит блокировку в tx; по pg_stat_activity тест
// дожидается, что запрос встал в ожидание, затем фиксирует tx и возвращает ответ. Без sleep.
func (s *SuppliersLiveSuite) race(hold func(ctx context.Context) (commit func()), req func() (int, []byte)) raceResult {
	ctx := context.Background()
	commit := hold(ctx)
	base := s.lockWaiters()
	done := make(chan raceResult, 1)
	go func() {
		st, raw := req()
		done <- raceResult{st, raw}
	}()
	s.Require().Eventually(func() bool { return s.lockWaiters() > base }, 10*time.Second, 20*time.Millisecond,
		"запрос ждёт блокировку")
	select {
	case res := <-done:
		s.FailNow("запрос не дождался блокировки", "%d %s", res.status, res.raw)
	default:
	}
	commit()
	return <-done
}

// DS-024 (§3 п. 2): удаление поставщика идёт (отметка поставлена, транзакция не зафиксирована) —
// второй запрос удаления обязан дождаться и получить 404 SUPPLIER_NOT_FOUND; отметка первого не
// перезаписана (UPDATE с условием «ещё не удалён»).
func (s *SuppliersLiveSuite) TestRace_SecondDeleteGets404() {
	sup := s.create(map[string]any{"name": "Гонка двойное " + sdMark()})
	admin := s.sdAdminID()

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE suppliers SET deleted_at = now(), deleted_by = $2, delete_reason = 'Первый' WHERE id = $1 AND deleted_at IS NULL`, sup.ID, admin)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) {
		st, _, raw := s.deleteSup(s.admin, sup.ID, "Второй")
		return st, raw
	})
	s.Equal(http.StatusNotFound, res.status, string(res.raw))
	s.Equal("SUPPLIER_NOT_FOUND", s.sdErr(res.raw).Code)
	s.Equal("true|true|Первый|"+admin, s.supState(sup.ID), "отметка первого удаления не перезаписана")
}

// §3 п. 4 (DS-017): создание поставщика с названием удалённого идёт (строка вставлена,
// транзакция не зафиксирована) — восстановление обязано дождаться на частичном уникальном индексе
// и получить 409 NAME_TAKEN: ровно один из двух получает название.
func (s *SuppliersLiveSuite) TestRace_RestoreWaitsForCreateSameName() {
	name := "Гонка имя " + sdMark()
	sup := s.create(map[string]any{"name": name})
	s.mustDeleteSup(sup.ID)

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `INSERT INTO suppliers (name) VALUES ($1)`, strings.ToUpper(name))
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.restoreSup(s.admin, sup.ID) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("NAME_TAKEN", s.sdErr(res.raw).Code)
	s.True(strings.HasPrefix(s.supState(sup.ID), "true|"), "поставщик остался удалённым")
}

// §3 п. 4 (DS-017): восстановление поставщика идёт (отметка стёрта, транзакция не
// зафиксирована) — создание поставщика с тем же названием обязано дождаться на индексе и получить
// 400 name: duplicate (as-is ответ создания).
func (s *SuppliersLiveSuite) TestRace_CreateWaitsForRestoreSameName() {
	name := "Гонка возврат " + sdMark()
	sup := s.create(map[string]any{"name": name})
	s.mustDeleteSup(sup.ID)

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE suppliers SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, sup.ID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.admin.Do(http.MethodPost, supAPI, map[string]any{"name": name}) })
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal(map[string]string{"name": "duplicate"}, s.fields(res.raw))
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM suppliers WHERE lower(btrim(name)) = lower($1)`, name), "поставщик с этим названием один")
}
