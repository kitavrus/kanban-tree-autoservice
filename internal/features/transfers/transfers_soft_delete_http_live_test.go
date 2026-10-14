//go:build integration

package transfers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление документов «Перемещение» (docs/features/soft-delete-transfers/draft-spec.md
// §11) — методы TransfersLiveSuite (§TD-8: боевой бинарь, реальный логин): TS-015 (документ
// удалённого заказа доводится до конца, новый не создаётся), TS-016 (причина, невидимость
// удалённого документа), TS-017 (восстановление заявки и занятая единица), TS-018 (гонка
// «создание документа ↔ удаление заказа»), TS-033 (права). Удаление по статусам (TS-016) —
// transfers_revert_http_live_test.go; заказ с документами (TS-014), смена клиента (TS-050) и
// служебная уборка (TS-051) — сьюта доски (internal/features/board).

// ---- хелперы ----

func (s *TransfersLiveSuite) mustDeleteDoc(t transferDTO) {
	st, raw := s.deleteDoc(t.ID, t.Version)
	s.Require().Equal(http.StatusOK, st, "delete #%d → %s", t.ID, raw)
}

func (s *TransfersLiveSuite) restoreDoc(id int64) (int, []byte) {
	return s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/restore", id), nil)
}

// softDeleteTask — мягкое удаление заказа через API доски того же сервера.
func (s *TransfersLiveSuite) softDeleteTask(b *testutil.Browser, taskID int64) (int, []byte) {
	return b.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{"reason": "Создан по ошибке"})
}

// docRow — «статус|удалён|версия» строки документа.
func (s *TransfersLiveSuite) docRow(id int64) string {
	return s.scalarText(`SELECT status || '|' || (deleted_at IS NOT NULL)::text || '|' || version::text FROM transfers WHERE id = $1`, id)
}

// ---- TS-015 ----

// TS-015 (DS-007): документ удалённого заказа доводится до конца как обычный — PUT, send,
// receive, unreceive, cancel → 200, в каждом ответе, в GET и в строке списка task_is_deleted =
// true (название заказа отдаётся как обычно); местоположение единицы меняется проведением как
// всегда. После восстановления заказа пометка исчезает. Негатив: POST /api/v1/transfers с
// task_id удалённого заказа → 409 TASK_DELETED (и администратору, и исполнителю; и «создать»,
// и «создать и отправить»), документа не появилось.
func (s *TransfersLiveSuite) TestTS015_TransferOfDeletedTask() {
	taskID, units := s.taskWithUnits(2)
	st, doc, raw := s.do(http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, units[0]))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.False(doc.TaskIsDeleted, "заказ действует")
	s.NotEmpty(doc.TaskTitle)

	st, raw = s.softDeleteTask(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	got := s.get(doc.ID)
	s.True(got.TaskIsDeleted, "GET: заказ удалён")
	s.Equal(doc.TaskTitle, got.TaskTitle, "название удалённого заказа отдаётся как обычно")
	s.Equal(doc.Version, got.Version, "удаление заказа документ не меняет")
	_, l := s.listIDs(s.admin, fmt.Sprintf("&task_id=%d", taskID))
	s.Require().Len(l.Items, 1)
	s.True(l.Items[0].TaskDeleted, "строка списка: task_is_deleted")
	_, l = s.listIDs(s.executor(), fmt.Sprintf("&task_id=%d", taskID))
	s.Require().Len(l.Items, 1, "исполнитель видит документ своего направления")
	s.True(l.Items[0].TaskDeleted)

	// негатив: новый документ по удалённому заказу не создаётся.
	before := s.transfersCount()
	for _, b := range []*testutil.Browser{s.admin, s.executor()} {
		for _, flags := range [][]any{nil, {"send", true}, {"receive", true}} {
			st, _, raw = s.doAs(b, http.MethodPost, "/api/v1/transfers", with(intakeBody(taskID, s.almaty, units[1]), flags...))
			s.expectConflict(st, raw, "TASK_DELETED", "Заказ удалён — сначала восстановите его", fmt.Sprint(flags))
		}
	}
	s.Equal(before, s.transfersCount(), "документа не появилось")
	s.Equal("at_client||", s.unitLoc(units[1]), "единица не занята")

	// правка заявки удалённого заказа (заказ у документа не меняется).
	st, edited, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", doc.ID), map[string]any{
		"to_warehouse_id": s.moscow, "note": "правка после удаления заказа", "items": unitItems(units[0]), "version": doc.Version,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("правка после удаления заказа", edited.Note)
	s.Equal(s.moscow, edited.ToWarehouseID)
	s.True(edited.TaskIsDeleted, "PUT")

	sent := s.must(edited, "send", nil)
	s.Equal("in_transit", sent.Status)
	s.True(sent.TaskIsDeleted, "send")
	s.Equal(fmt.Sprintf("in_transit||%d", doc.ID), s.unitLoc(units[0]))

	received := s.must(sent, "receive", nil)
	s.Equal("received", received.Status)
	s.True(received.TaskIsDeleted, "receive")
	s.Equal("at_warehouse|Москва|", s.unitLoc(units[0]), "приёмка меняет местоположение как обычно")

	back := s.mustUnreceive(received, "ошибочная приёмка")
	s.True(back.TaskIsDeleted, "unreceive")
	s.Equal(fmt.Sprintf("in_transit||%d", doc.ID), s.unitLoc(units[0]))

	cancelled := s.must(back, "cancel", nil)
	s.Equal("cancelled", cancelled.Status)
	s.True(cancelled.TaskIsDeleted, "cancel")
	s.Equal("at_client||", s.unitLoc(units[0]), "отмена вернула единицу клиенту")

	// заказ восстановили — пометка исчезает, документ не меняется.
	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", taskID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	after := s.get(doc.ID)
	s.False(after.TaskIsDeleted)
	s.Equal(cancelled.Version, after.Version)
	// …и новые документы по заказу снова создаются.
	st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, units[1]))
	s.Equal(http.StatusCreated, st, string(raw))

	// документ без заказа: task_is_deleted = false.
	s.False(s.create(s.body(unitItem(s.newUnit(s.almaty)))).TaskIsDeleted)
}

// ---- TS-016 (причина и невидимость) ----

// TS-016 (DS-020, DS-026, DS-030, abuse cases 2 и 5): причина удаления обязательна — нет /
// пробелы → required, длиннее 500 → too_long, NUL → invalid_text; ошибки причины и версии
// приходят вместе; отказ документ не трогает. Удалённый документ хранит «кто, когда, почему»
// (причина — после обрезки пробелов), версия +1; для обычных адресов он не существует: GET,
// PUT, send, receive, cancel, unreceive → 404 TRANSFER_NOT_FOUND; его нет в списке (и у
// исполнителя) и в истории единицы.
func (s *TransfersLiveSuite) TestTS016_SoftDeleteReasonAndInvisibility() {
	taskID, units := s.taskWithUnits(1)
	st, doc, raw := s.do(http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, units[0]))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	path := deletePath(doc.ID)

	for _, tc := range []struct {
		name string
		body any
		want map[string]string
	}{
		{"без причины", map[string]any{"version": doc.Version}, map[string]string{"reason": "required"}},
		{"причина null", map[string]any{"reason": nil, "version": doc.Version}, map[string]string{"reason": "required"}},
		{"причина из пробелов", map[string]any{"reason": " \t\n ", "version": doc.Version}, map[string]string{"reason": "required"}},
		{"причина 501 символ", map[string]any{"reason": strings.Repeat("я", 501), "version": doc.Version}, map[string]string{"reason": "too_long"}},
		{"причина с NUL", map[string]any{"reason": "a\x00b", "version": doc.Version}, map[string]string{"reason": "invalid_text"}},
		{"пустое тело", map[string]any{}, map[string]string{"reason": "required", "version": "required"}},
		{"без тела", nil, map[string]string{"reason": "required", "version": "required"}},
		{"версия 0", map[string]any{"reason": "x", "version": 0}, map[string]string{"version": "invalid"}},
		{"версия null", map[string]any{"reason": "x", "version": nil}, map[string]string{"version": "required"}},
	} {
		st, raw := s.admin.Do(http.MethodDelete, path, tc.body)
		s.Equal(http.StatusBadRequest, st, "%s: %s", tc.name, raw)
		s.Equal("VALIDATION", s.errOf(raw).Code, tc.name)
		s.Equal(tc.want, s.fields(raw), tc.name)
	}
	// прежний формат (version в query, без тела) больше не принимается.
	st, raw = s.admin.Do(http.MethodDelete, fmt.Sprintf("%s?version=%d", path, doc.Version), nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(fmt.Sprintf("requested|false|%d", doc.Version), s.docRow(doc.ID), "отказы документ не тронули")

	// причина 500 символов после обрезки пробелов — допустима.
	reason := strings.Repeat("я", 500)
	st, raw = s.admin.Do(http.MethodDelete, path, map[string]any{"reason": "  " + reason + " \n", "version": doc.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(fmt.Sprintf("requested|true|%d", doc.Version+1), s.docRow(doc.ID), "версия +1, статус прежний")
	s.Equal(s.adminID+"|"+reason, s.scalarText(`SELECT deleted_by::text || '|' || delete_reason FROM transfers WHERE id = $1`, doc.ID),
		"кто и почему (причина — после обрезки)")
	s.Equal("at_client||", s.unitLoc(units[0]), "единица заявки свободна, местоположение прежнее")

	// для обычных адресов документа нет (версия — уже актуальная: 404 не из-за неё).
	v := doc.Version + 1
	notFound := func(st int, raw []byte, ctx string) {
		s.Equal(http.StatusNotFound, st, "%s: %s", ctx, raw)
		e := s.errOf(raw)
		s.Equal("TRANSFER_NOT_FOUND", e.Code, ctx)
		s.Equal("Перемещение не найдено", e.Message, ctx)
	}
	docPath := fmt.Sprintf("/api/v1/transfers/%d", doc.ID)
	for _, b := range []*testutil.Browser{s.admin, s.executor()} {
		st, raw = b.Do(http.MethodGet, docPath, nil)
		notFound(st, raw, "GET")
		st, raw = b.Do(http.MethodPut, docPath, map[string]any{"to_warehouse_id": s.almaty, "items": unitItems(units[0]), "version": v})
		notFound(st, raw, "PUT")
		for _, act := range []string{"send", "receive", "cancel"} {
			st, raw = b.Do(http.MethodPost, docPath+"/"+act, map[string]any{"version": v})
			notFound(st, raw, act)
		}
	}
	st, raw = s.admin.Do(http.MethodPost, docPath+"/unreceive", map[string]any{"version": v, "reason": "x"})
	notFound(st, raw, "unreceive")
	st, raw = s.deleteDoc(doc.ID, v)
	notFound(st, raw, "повторное удаление")
	s.Equal(fmt.Sprintf("requested|true|%d", v), s.docRow(doc.ID), "404 документ не тронули")

	for _, b := range []*testutil.Browser{s.admin, s.executor()} {
		ids, _ := s.listIDs(b, fmt.Sprintf("&task_id=%d", taskID))
		s.False(ids[doc.ID], "удалённого документа нет в списке")
	}
	// история единицы: удалённый документ не показывается, после восстановления — снова есть.
	history := func() map[int64]bool {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment/"+units[0]+"/history", nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var h struct {
			Transfers []struct {
				ID int64 `json:"id"`
			} `json:"transfers"`
		}
		s.Require().NoError(json.Unmarshal(raw, &h), string(raw))
		out := map[int64]bool{}
		for _, t := range h.Transfers {
			out[t.ID] = true
		}
		return out
	}
	s.False(history()[doc.ID], "история единицы без удалённого документа")
	st, raw = s.restoreDoc(doc.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(history()[doc.ID])
	ids, _ := s.listIDs(s.admin, fmt.Sprintf("&task_id=%d", taskID))
	s.True(ids[doc.ID], "восстановленный документ снова в списке")
}

// ---- TS-017 ----

// TS-017 (DS-018, DS-024, DS-030): восстановление заявки — единицы снова заняты документом,
// строки на месте, отметка удаления стёрта, версия +1; остатки и местоположение не меняются
// (DS-008). Единица в другом незавершённом документе → 409 EQUIPMENT_IN_TRANSFER с его
// номером, документ остаётся удалённым, ничего не занято; после отмены мешающего документа
// восстановление проходит. Отменённый документ восстанавливается без занятия единиц. Негативы:
// повтор → 409 NOT_DELETED; действующий документ → 409 NOT_DELETED; нет документа → 404.
func (s *TransfersLiveSuite) TestTS017_RestoreRequest() {
	p := s.newProduct("20")
	u1, u2 := s.newUnit(s.almaty), s.newUnit(s.almaty)
	doc := s.create(s.body(productItem(p, 3), unitItem(u1), unitItem(u2)))
	s.mustDeleteDoc(doc)
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u1))

	st, raw := s.restoreDoc(doc.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var out struct {
		ID int64 `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	s.Equal(doc.ID, out.ID)
	restored := s.get(doc.ID)
	s.Equal("requested", restored.Status)
	s.Equal(doc.Version+2, restored.Version, "удаление и восстановление — по +1")
	s.Len(restored.Items, 3, "документ вернулся со своими строками")
	s.Equal("||", s.scalarText(`
		SELECT coalesce(deleted_at::text, '') || '|' || coalesce(deleted_by::text, '') || '|' || coalesce(delete_reason, '')
		FROM transfers WHERE id = $1`, doc.ID), "отметка удаления стёрта")
	for _, u := range []string{u1, u2} {
		s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", doc.ID), s.unitLoc(u), "единица снова занята документом")
	}
	s.Equal("20.000", s.pairStock(p, s.almaty), "остаток не менялся")
	s.Empty(s.movements(doc.ID), "движений нет")

	notDeleted := func(id int64, ctx string) {
		st, raw := s.restoreDoc(id)
		s.expectConflict(st, raw, "NOT_DELETED", "Запись уже восстановлена", ctx)
	}
	notDeleted(doc.ID, "повтор восстановления")
	s.Equal(doc.Version+2, s.get(doc.ID).Version, "повтор документ не тронул")
	for _, id := range []string{"999999999", "abc", "0"} {
		st, raw = s.admin.Do(http.MethodPost, "/api/v1/transfers/"+id+"/restore", nil)
		s.Equal(http.StatusNotFound, st, "%s: %s", id, raw)
		s.Equal("TRANSFER_NOT_FOUND", s.errOf(raw).Code)
	}
	// восстановленную заявку можно отправить — единицы на складе отправления и заняты ею.
	s.Equal("in_transit", s.must(restored, "send", nil).Status)

	// ---- единица занята другим незавершённым документом ----
	u3, u4 := s.newUnit(s.almaty), s.newUnit(s.almaty)
	gone := s.create(s.body(unitItem(u3), unitItem(u4)))
	s.mustDeleteDoc(gone)
	blocker := s.create(s.body(unitItem(u4)))
	st, raw = s.restoreDoc(gone.ID)
	e := s.expectConflict(st, raw, "EQUIPMENT_IN_TRANSFER", fmt.Sprintf("Оборудование уже в перемещении #%d", blocker.ID), "единица занята")
	s.Require().NotNil(e.Details, string(raw))
	s.Equal(blocker.ID, e.Details.TransferID)
	s.Equal(u4, e.Details.EquipmentID)
	s.Equal(fmt.Sprintf("requested|true|%d", gone.Version+1), s.docRow(gone.ID), "документ остался удалённым")
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u3), "свободная единица не занята: транзакция откатилась")
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", blocker.ID), s.unitLoc(u4))
	// мешающий документ отменён → восстановление проходит.
	s.must(blocker, "cancel", nil)
	st, raw = s.restoreDoc(gone.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", gone.ID), s.unitLoc(u3))
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", gone.ID), s.unitLoc(u4))

	// ---- отменённый документ: только очистка отметки ----
	u5 := s.newUnit(s.almaty)
	cancelled := s.must(s.create(s.body(unitItem(u5))), "cancel", nil)
	s.mustDeleteDoc(cancelled)
	other := s.create(s.body(unitItem(u5))) // единица отменённого документа свободна и занята другим
	st, raw = s.restoreDoc(cancelled.ID)
	s.Require().Equal(http.StatusOK, st, "отменённый восстанавливается и при занятой единице: %s", raw)
	s.Equal(fmt.Sprintf("cancelled|false|%d", cancelled.Version+2), s.docRow(cancelled.ID))
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", other.ID), s.unitLoc(u5), "единицу держит другой документ")
	notDeleted(other.ID, "действующий документ")
}

// TS-017 (§3 «Конкурентность»): два администратора одновременно восстанавливают две удалённые
// заявки с общей единицей — ровно одна 200, вторая 409 EQUIPMENT_IN_TRANSFER с номером первой;
// единица занята победителем, проигравшая остаётся удалённой; взаимной блокировки и 500 нет.
func (s *TransfersLiveSuite) TestTS017_RestoreRace_SharedUnit() {
	second := s.NewBrowser()
	second.Login("st-admin", "password1")
	browsers := []*testutil.Browser{s.admin, second}

	const iterations = 6
	for i := 0; i < iterations; i++ {
		shared := s.newUnit(s.almaty)
		// у каждой заявки ещё и своя единица: блокировки строк equipment берутся в порядке id.
		docs := make([]transferDTO, 2)
		for j := range docs {
			docs[j] = s.create(s.body(unitItem(shared), unitItem(s.newUnit(s.almaty))))
			s.mustDeleteDoc(docs[j])
		}
		statuses := make([]int, 2)
		bodies := make([][]byte, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for j := range docs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				statuses[j], bodies[j] = browsers[j].Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/restore", docs[j].ID), nil)
			}()
		}
		close(start)
		wg.Wait()

		winner, loser := 0, 1
		if statuses[0] != http.StatusOK {
			winner, loser = 1, 0
		}
		s.Require().Equal(http.StatusOK, statuses[winner], "итерация %d: %s / %s", i, bodies[0], bodies[1])
		s.expectConflict(statuses[loser], bodies[loser], "EQUIPMENT_IN_TRANSFER",
			fmt.Sprintf("Оборудование уже в перемещении #%d", docs[winner].ID), fmt.Sprintf("итерация %d", i))
		s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", docs[winner].ID), s.unitLoc(shared), "итерация %d", i)
		s.Equal(fmt.Sprintf("requested|false|%d", docs[winner].Version+2), s.docRow(docs[winner].ID), "итерация %d", i)
		s.Equal(fmt.Sprintf("requested|true|%d", docs[loser].Version+1), s.docRow(docs[loser].ID), "итерация %d", i)
		s.Equal("0", s.scalarText(`SELECT count(*)::text FROM equipment WHERE active_transfer_id = $1`, docs[loser].ID),
			"итерация %d: проигравшая заявка ничего не заняла", i)
	}
}

// TS-017 (ревью MEDIUM-1): заявка с заказом удалена, её единицу убрали из оборудования заказа →
// восстановление 409 TRANSFER_EQUIPMENT_NOT_IN_TASK с details.equipment_id; документ остаётся
// удалённым, ни одна единица не занята (в том числе оставшаяся в заказе). Единицу вернули в
// заказ → восстановление проходит. Негатив: отменённый документ с той же убранной единицей
// восстанавливается (единиц он не занимает) — проверка действует только на заявки.
func (s *TransfersLiveSuite) TestTS017_RestoreRequest_UnitRemovedFromTask() {
	taskID, units := s.taskWithUnits(3)
	kept, removed, third := units[0], units[1], units[2]
	st, doc, raw := s.do(http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, kept, removed))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, other, raw := s.do(http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, third))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	cancelled := s.must(other, "cancel", nil)
	s.mustDeleteDoc(doc)
	s.mustDeleteDoc(cancelled)

	// единицы удалённых документов свободны — убираются из заказа боевым API доски.
	for _, u := range []string{removed, third} {
		st, raw = s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, u), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
	}

	st, raw = s.restoreDoc(doc.ID)
	e := s.expectConflict(st, raw, "TRANSFER_EQUIPMENT_NOT_IN_TASK", "Оборудование убрано из заказа — восстановить перемещение нельзя", "единица вне заказа")
	s.Require().NotNil(e.Details, string(raw))
	s.Equal(removed, e.Details.EquipmentID)
	s.Equal(fmt.Sprintf("requested|true|%d", doc.Version+1), s.docRow(doc.ID), "документ остался удалённым")
	s.Equal("at_client||", s.unitLoc(kept), "оставшаяся в заказе единица не занята: транзакция откатилась")
	s.Equal("at_client||", s.unitLoc(removed))

	// негатив: отменённый документ единиц не занимает — восстанавливается и с убранной единицей.
	st, raw = s.restoreDoc(cancelled.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("at_client||", s.unitLoc(third))

	// единицу вернули в заказ — заявка восстанавливается и занимает обе единицы.
	s.attach(taskID, removed)
	st, raw = s.restoreDoc(doc.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	for _, u := range []string{kept, removed} {
		s.Equal(fmt.Sprintf("at_client||%d", doc.ID), s.unitLoc(u))
	}
}

// ---- TS-018 ----

// TS-018 (DS-007, DS-024): гонка «создание документа ↔ удаление заказа» — исход детерминирован:
// удаление заказа всегда 200; создание — либо 201 (документ остался у удалённого заказа,
// task_is_deleted = true, единица занята им), либо 409 TASK_DELETED. Взаимной блокировки и 500
// нет. Негатив: частично созданного документа нет — при отказе у заказа нет ни документа, ни
// строк, единица свободна.
func (s *TransfersLiveSuite) TestTS018_CreateVsDeleteTaskRace() {
	second := s.NewBrowser()
	second.Login("st-admin", "password1")

	const iterations = 12
	created, refused := 0, 0
	for i := 0; i < iterations; i++ {
		taskID, units := s.taskWithUnits(1)
		var (
			createSt, deleteSt   int
			createRaw, deleteRaw []byte
			wg                   sync.WaitGroup
		)
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			createSt, createRaw = s.admin.Do(http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, units[0]))
		}()
		go func() {
			defer wg.Done()
			<-start
			deleteSt, deleteRaw = s.softDeleteTask(second, taskID)
		}()
		close(start)
		wg.Wait()

		s.Require().Equal(http.StatusOK, deleteSt, "итерация %d: удаление заказа → %s", i, deleteRaw)
		docs := s.scalarText(`SELECT count(*)::text FROM transfers WHERE task_id = $1`, taskID)
		items := s.scalarText(`SELECT count(*)::text FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id WHERE t.task_id = $1`, taskID)
		switch createSt {
		case http.StatusCreated:
			created++
			var doc transferDTO
			s.Require().NoError(json.Unmarshal(createRaw, &doc), string(createRaw))
			s.Equal("1|1", docs+"|"+items, "итерация %d: документ создан целиком", i)
			s.Equal(fmt.Sprintf("at_client||%d", doc.ID), s.unitLoc(units[0]), "итерация %d", i)
			s.True(s.get(doc.ID).TaskIsDeleted, "итерация %d: документ остался у удалённого заказа", i)
		case http.StatusConflict:
			refused++
			s.expectConflict(createSt, createRaw, "TASK_DELETED", "Заказ удалён — сначала восстановите его", fmt.Sprintf("итерация %d", i))
			s.Equal("0|0", docs+"|"+items, "итерация %d: частично созданного документа нет", i)
			s.Equal("at_client||", s.unitLoc(units[0]), "итерация %d: единица свободна", i)
		default:
			s.Failf("недетерминированный исход", "итерация %d: создание → %d %s", i, createSt, createRaw)
		}
	}
	s.Equal(iterations, created+refused)
	s.T().Logf("исходы гонки: документ создан — %d, отказ TASK_DELETED — %d", created, refused)
}

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): удаление и восстановление документа — только администратор.
// Исполнитель получает 403 «Нет доступа» до поиска записи: для документа своего направления
// (с клиентом), чужого, удалённого, несуществующего и нечислового номера, с телом и без;
// документы не меняются. Негатив: без сессии → 401.
func (s *TransfersLiveSuite) TestTS033_SoftDeleteAdminOnly() {
	exec := s.executor()
	taskID, units := s.taskWithUnits(2)
	st, own, raw := s.doAs(exec, http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, units[0]))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, ownGone, raw := s.doAs(exec, http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, units[1]))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.mustDeleteDoc(ownGone)
	w2w := s.create(s.body(unitItem(s.newUnit(s.almaty))))
	anon := s.NewBrowser()

	type call struct {
		method, path string
		body         any
	}
	calls := []call{
		{http.MethodDelete, deletePath(own.ID), deleteBody(own.Version)},
		{http.MethodDelete, deletePath(own.ID), nil}, // роль проверяется раньше тела
		{http.MethodDelete, deletePath(w2w.ID), deleteBody(w2w.Version)},
		{http.MethodDelete, deletePath(ownGone.ID), deleteBody(ownGone.Version + 1)},
		{http.MethodDelete, deletePath(999999999), deleteBody(1)},
		{http.MethodDelete, "/api/v1/transfers/abc", deleteBody(1)},
		{http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/restore", ownGone.ID), nil},
		{http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/restore", own.ID), nil},
		{http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/restore", w2w.ID), nil},
		{http.MethodPost, "/api/v1/transfers/999999999/restore", nil},
		{http.MethodPost, "/api/v1/transfers/abc/restore", nil},
	}
	for _, c := range calls {
		st, raw := exec.Do(c.method, c.path, c.body)
		s.Equal(http.StatusForbidden, st, "%s %s: %s", c.method, c.path, raw)
		e := s.errOf(raw)
		s.Equal("FORBIDDEN", e.Code, c.path)
		s.Equal("Нет доступа", e.Message, c.path)

		st, raw = anon.Do(c.method, c.path, c.body)
		s.Equal(http.StatusUnauthorized, st, "без сессии %s %s: %s", c.method, c.path, raw)
	}
	s.Equal(fmt.Sprintf("requested|false|%d", own.Version), s.docRow(own.ID))
	s.Equal(fmt.Sprintf("requested|false|%d", w2w.Version), s.docRow(w2w.ID))
	s.Equal(fmt.Sprintf("requested|true|%d", ownGone.Version+1), s.docRow(ownGone.ID))

	// администратору те же действия доступны.
	s.mustDeleteDoc(own)
	st, raw = s.restoreDoc(ownGone.ID)
	s.Equal(http.StatusOK, st, string(raw))
}
