package softdelete

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ClientsSource — удалённые клиенты (вкладка «Клиенты», часть 4 soft-delete-clients). Читает
// таблицу clients напрямую: softdelete — reader, отметку удаления пишет пакет clients.
type ClientsSource struct {
	pool *pgxpool.Pool
}

func NewClientsSource(pool *pgxpool.Pool) *ClientsSource { return &ClientsSource{pool: pool} }

// clientsWhereSQL — фильтр вкладки «Клиенты»: $1 — строка поиска (название компании, причина;
// уже экранирована), $2/$3 — период [с; по), $4 — кто удалил. Номера у клиента нет. Частичный
// индекс clients_deleted_at_idx покрывает условие и сортировку.
const clientsWhereSQL = `
FROM clients c
JOIN users u ON u.id = c.deleted_by
WHERE c.deleted_at IS NOT NULL
  AND ($1::text IS NULL OR c.company_name ILIKE '%' || $1 || '%' ESCAPE '\'
       OR c.delete_reason ILIKE '%' || $1 || '%' ESCAPE '\')
  AND ($2::timestamptz IS NULL OR c.deleted_at >= $2)
  AND ($3::timestamptz IS NULL OR c.deleted_at < $3)
  AND ($4::uuid IS NULL OR c.deleted_by = $4)`

// List — строка клиента: title — название компании, subtitle — пустая строка (§6.5).
func (s *ClientsSource) List(ctx context.Context, f Filter) ([]Item, int, error) {
	args := filterArgs(f)

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+clientsWhereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT c.id::text, c.company_name, c.deleted_at, u.id::text, coalesce(nullif(u.name, ''), u.login), c.delete_reason`+
		clientsWhereSQL+`
ORDER BY c.deleted_at DESC, c.id
LIMIT $5 OFFSET $6`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		it := Item{Kind: string(KindClients)}
		if err := rows.Scan(&it.ID, &it.Title, &it.DeletedAt, &it.DeletedBy.ID, &it.DeletedBy.Name, &it.DeleteReason); err != nil {
			return nil, 0, err
		}
		it.DeletedAt = it.DeletedAt.UTC()
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (s *ClientsSource) Count(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM clients WHERE deleted_at IS NOT NULL`).Scan(&n)
	return n, err
}

// filterArgs — общие параметры $1..$4 фильтра вкладок клиентов и оборудования: пустые строки —
// NULL («без фильтра»).
func filterArgs(f Filter) []any {
	var q, by any
	if f.Q != "" {
		q = f.Q
	}
	if f.DeletedBy != "" {
		by = f.DeletedBy
	}
	return []any{q, f.DeletedFrom, f.DeletedTo, by}
}

// EquipmentSource — удалённые единицы оборудования (вкладка «Оборудование», часть 4
// soft-delete-clients). Читает equipment напрямую; отметку удаления пишет пакет clients.
type EquipmentSource struct {
	pool *pgxpool.Pool
}

func NewEquipmentSource(pool *pgxpool.Pool) *EquipmentSource { return &EquipmentSource{pool: pool} }

// DeletedWithClient — значение deleted_with у единицы, ушедшей вместе с клиентом (DS-015).
const DeletedWithClient = "client"

// EquipmentTitle — заголовок строки удалённой единицы: «тип марка модель» (§6.5); пустая марка
// пропускается. Непустой госномер дописывается через « · » («Легковой Lada Vesta · А123ВС 777»).
func EquipmentTitle(typeName, brand, model, plateNumber string) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{typeName, brand, model} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	title := strings.Join(parts, " ")
	if plate := strings.TrimSpace(plateNumber); plate != "" {
		title += " · " + plate
	}
	return title
}

// EquipmentDeletedWith — deleted_with строки единицы: "client" у ушедшей вместе с клиентом,
// иначе nil (JSON null).
func EquipmentDeletedWith(withClient bool) *string {
	if !withClient {
		return nil
	}
	v := DeletedWithClient
	return &v
}

// equipmentWhereSQL — фильтр вкладки «Оборудование»: $1 — строка поиска (тип, марка, модель,
// серийный номер, госномер — без пробелов/дефисов с обеих сторон, название клиента, причина; уже экранирована), $2/$3 — период, $4 — кто
// удалил. Клиент и тип соединяются без условия «не удалён»: клиент единицы сам может быть в
// удалённых. Частичный индекс equipment_deleted_at_idx покрывает условие и сортировку.
const equipmentWhereSQL = `
FROM equipment e
JOIN equipment_types t ON t.id = e.type_id
JOIN clients c         ON c.id = e.client_id
JOIN users u           ON u.id = e.deleted_by
WHERE e.deleted_at IS NOT NULL
  AND ($1::text IS NULL OR t.name ILIKE '%' || $1 || '%' ESCAPE '\'
       OR e.brand ILIKE '%' || $1 || '%' ESCAPE '\'
       OR e.model ILIKE '%' || $1 || '%' ESCAPE '\'
       OR e.serial ILIKE '%' || $1 || '%' ESCAPE '\'
       OR replace(replace(e.plate_number, ' ', ''), '-', '')
          ILIKE '%' || replace(replace($1, ' ', ''), '-', '') || '%' ESCAPE '\'
       OR c.company_name ILIKE '%' || $1 || '%' ESCAPE '\'
       OR e.delete_reason ILIKE '%' || $1 || '%' ESCAPE '\')
  AND ($2::timestamptz IS NULL OR e.deleted_at >= $2)
  AND ($3::timestamptz IS NULL OR e.deleted_at < $3)
  AND ($4::uuid IS NULL OR e.deleted_by = $4)`

// List — строка единицы: title — «тип марка модель», subtitle — название клиента,
// deleted_with — "client" у ушедших вместе с клиентом (§6.5, DS-015).
func (s *EquipmentSource) List(ctx context.Context, f Filter) ([]Item, int, error) {
	args := filterArgs(f)

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+equipmentWhereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT e.id::text, t.name, e.brand, e.model, e.plate_number, c.company_name, e.deleted_with_client,
       e.deleted_at, u.id::text, coalesce(nullif(u.name, ''), u.login), e.delete_reason`+
		equipmentWhereSQL+`
ORDER BY e.deleted_at DESC, e.id
LIMIT $5 OFFSET $6`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		var typeName, brand, model, plate string
		var withClient bool
		it := Item{Kind: string(KindEquipment)}
		if err := rows.Scan(&it.ID, &typeName, &brand, &model, &plate, &it.Subtitle, &withClient,
			&it.DeletedAt, &it.DeletedBy.ID, &it.DeletedBy.Name, &it.DeleteReason); err != nil {
			return nil, 0, err
		}
		it.Title = EquipmentTitle(typeName, brand, model, plate)
		it.DeletedWith = EquipmentDeletedWith(withClient)
		it.DeletedAt = it.DeletedAt.UTC()
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (s *EquipmentSource) Count(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM equipment WHERE deleted_at IS NOT NULL`).Scan(&n)
	return n, err
}
