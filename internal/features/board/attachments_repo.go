package board

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AttachmentRepo — метаданные вложений заказа (task_attachments, миграция 0007). Свой pool,
// без блокировки заказа: вложения не трогают склад/итоги/version заказа (как журнал работ).
// FK-проверка INSERT берёт KEY SHARE на tasks — параллельный DELETE заказа (FOR UPDATE)
// дожидается либо, наоборот, INSERT падает 23503 (→ errNotFound).
type AttachmentRepo struct {
	pool *pgxpool.Pool
}

func NewAttachmentRepo(pool *pgxpool.Pool) *AttachmentRepo { return &AttachmentRepo{pool: pool} }

// attachmentRow — строка task_attachments (+ имя автора для карточки).
type attachmentRow struct {
	ID           string
	TaskID       int64
	Kind         string
	OriginalName string
	ContentType  string
	Size         int64
	StorageKey   string
	UserID       string
	AuthorName   string
	CreatedAt    time.Time
}

// TaskExists — действующий заказ есть; мягко удалённый — как несуществующий (soft-delete-core
// DS-026) (проверка ДО чтения файла из тела запроса: несуществующий заказ
// не должен заставлять сервер писать сотни мегабайт на диск).
func (r *AttachmentRepo) TaskExists(ctx context.Context, taskID int64) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tasks WHERE id = $1 AND deleted_at IS NULL)`, taskID).Scan(&ok)
	return ok, err
}

// Insert — строка вложения действующего заказа. Заказ блокируется FOR KEY SHARE на время
// оператора (soft-delete-core DS-026): мягкое удаление (LockTask, FOR UPDATE) либо ждёт конца
// вставки, либо вставка ждёт его и строки заказа уже не видит → 0 строк → errNotFound.
// 23503 на task_id — заказ физически удалён между TaskExists и INSERT → errNotFound.
func (r *AttachmentRepo) Insert(ctx context.Context, in attachmentRow) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO task_attachments (task_id, kind, original_name, content_type, size, storage_key, user_id)
		SELECT t.id, $2, $3, $4, $5, $6, $7 FROM tasks t WHERE t.id = $1 AND t.deleted_at IS NULL
		FOR KEY SHARE OF t
		RETURNING id`,
		in.TaskID, in.Kind, in.OriginalName, in.ContentType, in.Size, in.StorageKey, in.UserID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "task_attachments_task_id_fkey" {
			return "", errNotFound
		}
		return "", err
	}
	return id, nil
}

const attachmentSelectSQL = `
	SELECT a.id, a.task_id, a.kind, a.original_name, a.content_type, a.size, a.storage_key, a.user_id, u.name, a.created_at
	FROM task_attachments a JOIN users u ON u.id = a.user_id`

func scanAttachment(row pgx.Row, a *attachmentRow) error {
	return row.Scan(&a.ID, &a.TaskID, &a.Kind, &a.OriginalName, &a.ContentType, &a.Size, &a.StorageKey,
		&a.UserID, &a.AuthorName, &a.CreatedAt)
}

// Get — одна строка по id. 0 → errNotFound — в том числе вложение мягко удалённого заказа
// (soft-delete-core DS-013: файл удалённого заказа не отдаётся и не удаляется поштучно).
func (r *AttachmentRepo) Get(ctx context.Context, id string) (*attachmentRow, error) {
	var a attachmentRow
	err := scanAttachment(r.pool.QueryRow(ctx, attachmentSelectSQL+`
		JOIN tasks t ON t.id = a.task_id WHERE a.id = $1 AND t.deleted_at IS NULL`, id), &a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// List — вложения заказа в порядке загрузки (карточка, AC фазы 7).
func (r *AttachmentRepo) List(ctx context.Context, taskID int64) ([]attachmentRow, error) {
	rows, err := r.pool.Query(ctx, attachmentSelectSQL+` WHERE a.task_id = $1 ORDER BY a.created_at, a.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []attachmentRow
	for rows.Next() {
		var a attachmentRow
		if err := scanAttachment(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Delete — удаление строки вложения одной транзакцией под блокировкой его заказа
// (soft-delete-core DS-013, DS-026): строка читается с FOR KEY SHARE заказа и условием «заказ
// не удалён» (0 строк → errNotFound — вложения нет либо заказ мягко удалён, в т.ч. параллельно:
// удаление заказа берёт FOR UPDATE и с KEY SHARE несовместимо), allow проверяет права, затем
// DELETE. Возвращает удалённую строку — файл с диска сервис стирает ТОЛЬКО после успешного
// commit. Блокировка одна (tasks) — взаимных блокировок с Delete / Restore заказа нет.
func (r *AttachmentRepo) Delete(ctx context.Context, id string, allow func(attachmentRow) error) (*attachmentRow, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var a attachmentRow
	err = scanAttachment(tx.QueryRow(ctx, attachmentSelectSQL+`
		JOIN tasks t ON t.id = a.task_id WHERE a.id = $1 AND t.deleted_at IS NULL FOR KEY SHARE OF t`, id), &a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := allow(a); err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM task_attachments WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, errNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &a, nil
}

// attachmentKeysTx — ключи файлов заказа внутри транзакции удаления заказа (строки уйдут
// каскадом FK, файлы удаляет сервис ПОСЛЕ commit). Заказ уже под FOR UPDATE — новый
// INSERT вложения ждёт commit и падает 23503, так что список полон.
func attachmentKeysTx(ctx context.Context, tx pgx.Tx, taskID int64) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT storage_key FROM task_attachments WHERE task_id = $1`, taskID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// ExistingKeys — подмножество storage_key, для которых есть строка (filestore.KeyIndex для
// сборщика файлов-сирот). Поиск по UNIQUE-индексу storage_key. Условия «заказ не удалён»
// здесь НЕТ намеренно (soft-delete-core DS-013): файлы мягко удалённого заказа — не сироты.
func (r *AttachmentRepo) ExistingKeys(ctx context.Context, keys []string) (map[string]struct{}, error) {
	rows, err := r.pool.Query(ctx, `SELECT storage_key FROM task_attachments WHERE storage_key = ANY($1)`, keys)
	if err != nil {
		return nil, fmt.Errorf("attachments existing keys: %w", err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("attachments existing keys: %w", err)
	}
	out := make(map[string]struct{}, len(found))
	for _, k := range found {
		out[k] = struct{}{}
	}
	return out, nil
}
