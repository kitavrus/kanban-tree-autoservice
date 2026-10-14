package catalog

import (
	"net/url"
	"strconv"
)

// ssrPageSize — pagination DS-005: SSR-списки товаров и услуг выдаются страницами по 100
// позиций (параметр URL `offset`, не `page` — §API-1). Лимиты API (defaultLimit/maxLimit)
// не меняются (DS-008).
const ssrPageSize = 100

// PagerVM — пейджер-partial «catalogPager» (web/ssr/catalog/templates/partials/
// catalog-pager.html): «Показаны 101–200 из 2345» + ссылки «‹ Назад»/«Вперёд ›».
type PagerVM struct {
	Show    bool // пейджер нужен: позиций больше одной страницы или открыта не первая
	From    int  // 1-based номер первой строки страницы (0 — строк нет)
	To      int
	Total   int
	PrevURL string // "" — ссылки нет
	NextURL string
}

// parseSSROffset — pagination DS-005: offset кратен ssrPageSize; невалидный/отрицательный
// → 0, некратный округляется вниз до начала страницы.
func parseSSROffset(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n - n%ssrPageSize
}

// buildPager — ссылки пейджера сохраняют фильтры q/category/archived (DS-005); смена
// фильтра/поиска идёт через форму/ссылки без offset — сброс на первую страницу. offset за
// пределами total → пустая страница, «Назад» ведёт на последнюю непустую (не 404).
func buildPager(path, q, category string, archived bool, offset, shown, total int) PagerVM {
	p := PagerVM{Total: total, Show: total > ssrPageSize || offset > 0}
	if shown > 0 {
		p.From, p.To = offset+1, offset+shown
	}
	link := func(off int) string {
		v := url.Values{}
		if q != "" {
			v.Set("q", q)
		}
		if category != "" {
			v.Set("category", category)
		}
		if archived {
			v.Set("archived", "1")
		}
		if off > 0 {
			v.Set("offset", strconv.Itoa(off))
		}
		if len(v) == 0 {
			return path
		}
		return path + "?" + v.Encode()
	}
	if offset > 0 {
		prev := offset - ssrPageSize
		if offset >= total { // за пределами выдачи — назад на последнюю непустую страницу
			prev = 0
			if total > 0 {
				prev = (total - 1) / ssrPageSize * ssrPageSize
			}
		}
		p.PrevURL = link(max(prev, 0))
	}
	if offset+shown < total && shown > 0 {
		p.NextURL = link(offset + ssrPageSize)
	}
	return p
}

// TruncVM — pagination DS-006: категории — lookup с лимитом defaultLimit; при total >
// len(items) шаблон рендерит пометку «показаны N из M» (data-testid="categories-truncated").
type TruncVM struct {
	Shown int
	Total int
}

// Truncated — есть ли что показать в пометке.
func (t TruncVM) Truncated() bool { return t.Total > t.Shown }
