// Package filestore — хранилище двоичных файлов (вложения заказов, фаза 7/D21) за узким
// интерфейсом FileStore: сейчас локальный диск (LocalFileStore, каталог UPLOADS_DIR),
// позже MinIO/S3 — новой реализацией без переписывания вызывающего кода.
package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrTooLarge — поток длиннее maxBytes (Put); частично записанный файл удалён.
	ErrTooLarge = errors.New("filestore: file too large")
	// ErrNotFound — ключа нет в хранилище (Open).
	ErrNotFound = errors.New("filestore: not found")
	// ErrInvalidKey — ключ не uuid (защита от path traversal: имя пользователя в путь не
	// попадает никогда, ключ генерирует сервер).
	ErrInvalidKey = errors.New("filestore: invalid key")
)

// Info — метаданные открытого файла (для Content-Length/Last-Modified/Range).
type Info struct {
	Size    int64
	ModTime time.Time
}

// FileStore — контракт хранилища. Ключ — uuid, выдаётся вызывающей стороной (NewKey).
type FileStore interface {
	// Put пишет поток под ключом; больше maxBytes → ErrTooLarge (ничего не остаётся).
	// Возвращает число записанных байт.
	Put(ctx context.Context, key string, r io.Reader, maxBytes int64) (int64, error)
	// Open — поток с произвольным доступом (Range) + размер/время. Закрывает вызывающий.
	Open(ctx context.Context, key string) (io.ReadSeekCloser, Info, error)
	// Delete — удалить; отсутствующий ключ — не ошибка (идемпотентно).
	Delete(ctx context.Context, key string) error
}

// NewKey — ключ хранения нового файла (uuid v4, не имя пользователя).
func NewKey() string { return uuid.NewString() }

// LocalFileStore — файлы плоско в каталоге dir, имя файла = ключ.
type LocalFileStore struct {
	dir string
}

// NewLocal создаёт каталог (0750), если его нет.
func NewLocal(dir string) (*LocalFileStore, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("filestore: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("filestore: mkdir %s: %w", abs, err)
	}
	return &LocalFileStore{dir: abs}, nil
}

// Dir — абсолютный путь каталога (диагностика/тесты).
func (s *LocalFileStore) Dir() string { return s.dir }

func (s *LocalFileStore) path(key string) (string, error) {
	if uuid.Validate(key) != nil {
		return "", ErrInvalidKey
	}
	return filepath.Join(s.dir, key), nil
}

// Put — запись во временный файл того же каталога + rename (читатель никогда не видит
// недописанный файл; сбой посередине не оставляет файл под целевым ключом).
func (s *LocalFileStore) Put(ctx context.Context, key string, r io.Reader, maxBytes int64) (n int64, err error) {
	dst, err := s.path(key)
	if err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(s.dir, tempPrefix+"*")
	if err != nil {
		return 0, fmt.Errorf("filestore: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	n, err = io.Copy(tmp, io.LimitReader(ctxReader{ctx: ctx, r: r}, maxBytes+1))
	if err != nil {
		return 0, err
	}
	if n > maxBytes {
		return 0, ErrTooLarge
	}
	if err = tmp.Close(); err != nil {
		return 0, fmt.Errorf("filestore: close: %w", err)
	}
	if err = os.Rename(tmpName, dst); err != nil {
		return 0, fmt.Errorf("filestore: rename: %w", err)
	}
	return n, nil
}

func (s *LocalFileStore) Open(_ context.Context, key string) (io.ReadSeekCloser, Info, error) {
	p, err := s.path(key)
	if err != nil {
		return nil, Info{}, err
	}
	f, err := os.Open(p) //nolint:gosec // p — каталог хранилища + uuid-ключ (path() валидирует)
	if errors.Is(err, os.ErrNotExist) {
		return nil, Info{}, ErrNotFound
	}
	if err != nil {
		return nil, Info{}, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, Info{}, err
	}
	return f, Info{Size: st.Size(), ModTime: st.ModTime()}, nil
}

func (s *LocalFileStore) Delete(_ context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ctxReader — отмена запроса (клиент оборвал загрузку) прерывает копирование.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
