//go:build integration

package board_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ---- pagination DS-002: GET /api/v1/tasks — column_id, per_column, column_total ----
// Методы BoardLiveSuite (тот же suite.Run, §TD-8): реальный бинарь + TCP + логин. Каждый
// тест заводит своего клиента и фильтрует по client_id — чужие заказы suite не мешают.

type pgCardDTO struct {
	ID          int64  `json:"id"`
	ColumnID    string `json:"column_id"`
	Title       string `json:"title"`
	ColumnTotal *int   `json:"column_total"` // указатель — проверяем, что ключ присутствует
}

type pgListDTO struct {
	Items  []pgCardDTO `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

func (s *BoardLiveSuite) pgList(query url.Values) pgListDTO {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?"+query.Encode(), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l pgListDTO
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	return l
}

// pgTasks — n заказов клиента в колонке, в порядке создания (новый заказ — в конец
// колонки, т.е. порядок position,id = порядок создания).
func (s *BoardLiveSuite) pgTasks(clientID, columnID, prefix string, n int) []int64 {
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks", map[string]any{
			"title": fmt.Sprintf("%s %d", prefix, i+1), "client_id": clientID, "column_id": columnID,
		})
		s.Require().Equal(http.StatusCreated, st, string(raw))
		ids = append(ids, m.Task.ID)
	}
	return ids
}

// TS-002: per_column=2 на колонке с 5 заказами → 2 карточки этой колонки, column_total=5
// у каждой; total — все подходящие под фильтр (без окна per_column).
func (s *BoardLiveSuite) TestBoardLiveSuite_Pagination_PerColumnWindow_TS002() {
	c := s.newClient("Клиент-TS-002")
	colA := s.columnID("Новые")
	colB := s.columnID("Согласование")
	idsA := s.pgTasks(c, colA, "TS-002 A", 5)
	idsB := s.pgTasks(c, colB, "TS-002 B", 1)

	l := s.pgList(url.Values{"client_id": {c}, "per_column": {"2"}, "limit": {"1000"}})
	s.Equal(6, l.Total, "total — без окна per_column")
	s.Equal(1000, l.Limit)

	var gotA, gotB []int64
	for _, it := range l.Items {
		s.Require().NotNil(it.ColumnTotal, "column_total всегда присутствует")
		switch it.ColumnID {
		case colA:
			gotA = append(gotA, it.ID)
			s.Equal(5, *it.ColumnTotal)
		case colB:
			gotB = append(gotB, it.ID)
			s.Equal(1, *it.ColumnTotal)
		}
	}
	s.Equal(idsA[:2], gotA, "первые 2 по position,id")
	s.Equal(idsB, gotB)
	s.Len(l.Items, 3)

	// без per_column — прежний контракт: все карточки, column_total тоже заполнен.
	l = s.pgList(url.Values{"client_id": {c}})
	s.Equal(6, l.Total)
	s.Len(l.Items, 6)
	for _, it := range l.Items {
		s.Require().NotNil(it.ColumnTotal)
	}
}

// TS-003: column_id=X&limit=2&offset=2 → карточки 3–4 колонки X в порядке position,id;
// total=5 (с учётом column_id).
func (s *BoardLiveSuite) TestBoardLiveSuite_Pagination_ColumnIDLimitOffset_TS003() {
	c := s.newClient("Клиент-TS-003")
	colA := s.columnID("Новые")
	ids := s.pgTasks(c, colA, "TS-003", 5)
	s.pgTasks(c, s.columnID("Согласование"), "TS-003 other", 2)

	l := s.pgList(url.Values{"client_id": {c}, "column_id": {colA}, "limit": {"2"}, "offset": {"2"}})
	s.Equal(5, l.Total)
	s.Equal(2, l.Offset)
	s.Require().Len(l.Items, 2)
	s.Equal(ids[2], l.Items[0].ID)
	s.Equal(ids[3], l.Items[1].ID)
	for _, it := range l.Items {
		s.Equal(colA, it.ColumnID)
		s.Require().NotNil(it.ColumnTotal)
		s.Equal(5, *it.ColumnTotal)
	}

	// хвост: offset=4 → 1 карточка (последняя).
	l = s.pgList(url.Values{"client_id": {c}, "column_id": {colA}, "limit": {"50"}, "offset": {"4"}})
	s.Require().Len(l.Items, 1)
	s.Equal(ids[4], l.Items[0].ID)

	// невалидные параметры → 400 VALIDATION по своему полю (DS-002).
	for q, field := range map[string]string{
		"column_id=bad": "column_id", "per_column=0": "per_column", "per_column=1001": "per_column", "per_column=x": "per_column",
	} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?"+q, nil)
		s.Require().Equal(http.StatusBadRequest, st, "%s: %s", q, raw)
		s.Equal("VALIDATION", s.errOf(raw).Code)
		s.Contains(s.fields(raw), field, q)
	}
}

// TS-004: фильтр q + per_column → column_total считается с учётом фильтра.
func (s *BoardLiveSuite) TestBoardLiveSuite_Pagination_ColumnTotalRespectsFilter_TS004() {
	c := s.newClient("Клиент-TS-004")
	colA := s.columnID("Новые")
	s.pgTasks(c, colA, "TS004match", 3)
	s.pgTasks(c, colA, "TS004other", 4)

	l := s.pgList(url.Values{"client_id": {c}, "q": {"TS004match"}, "per_column": {"1"}})
	s.Equal(3, l.Total)
	s.Require().Len(l.Items, 1)
	s.Require().NotNil(l.Items[0].ColumnTotal)
	s.Equal(3, *l.Items[0].ColumnTotal)
	s.Contains(l.Items[0].Title, "TS004match")

	l = s.pgList(url.Values{"client_id": {c}, "per_column": {"1"}})
	s.Equal(7, l.Total)
	s.Require().Len(l.Items, 1)
	s.Equal(7, *l.Items[0].ColumnTotal)
}
