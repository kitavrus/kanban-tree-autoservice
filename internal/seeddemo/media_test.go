package seeddemo

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDemoData_Valid — госномера (кириллица АВЕКМНОРСТУХ, «А123ВС 777»), VIN и ссылки
// заказов на авто/услуги/товары корректны.
func TestDemoData_Valid(t *testing.T) {
	require.NoError(t, validateDemoData())
	require.Regexp(t, PlateRe, "А123ВС 777")
	require.NotRegexp(t, PlateRe, "A123BC 777", "латиница вместо кириллицы")
}

// TestFont_CoversCaptions — все символы подписей и шапок вложений есть в шрифте.
func TestFont_CoversCaptions(t *testing.T) {
	var texts []string
	for _, tk := range demoTasks {
		for _, m := range tk.media {
			texts = append(texts, m.caption, m.codes)
		}
	}
	for _, e := range demoEquipment {
		texts = append(texts, e.plate, e.specValue("Пробег"), "ЗАКАЗ #12 · ДВС "+e.specValue("Код двигателя"))
	}
	texts = append(texts, "ФОТО", "CHECK", "RUS", "MIN", "MAX", "REC", "X1000", "KM/H")
	for _, s := range texts {
		for _, r := range s {
			_, ok := glyphFor(r)
			require.Truef(t, ok, "нет глифа %q в %q", r, s)
		}
	}
}

// TestRenderPhoto_JPEG — фото декодируется и распознаётся как image/jpeg (как при загрузке).
func TestRenderPhoto_JPEG(t *testing.T) {
	data, err := renderPhoto("ЗАКАЗ #1 · О718МР 199", "ФОТО ПРИЁМКИ", sceneCar("О718МР 199", "белый", false))
	require.NoError(t, err)
	require.Equal(t, "image/jpeg", http.DetectContentType(data))
	img, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, photoW, img.Bounds().Dx())
	writeSample(t, "photo.jpg", data)
}

// TestRenderVideo_MP4 — видео распознаётся как video/mp4, структура ISO BMFF цела:
// ftyp → moov → mdat, stco указывает на начало данных mdat.
func TestRenderVideo_MP4(t *testing.T) {
	data := renderVideo("ЗАКАЗ #1 · О718МР 199", "ТРОИТ НА ХОЛОДНУЮ", sceneEngine(0.4, 0.4, true, false))
	require.Equal(t, "video/mp4", http.DetectContentType(data))
	var types []string
	var mdatData int
	for off := 0; off < len(data); {
		size := int(binary.BigEndian.Uint32(data[off:]))
		require.Positive(t, size)
		typ := string(data[off+4 : off+8])
		types = append(types, typ)
		if typ == "mdat" {
			mdatData = off + 8
		}
		off += size
		require.LessOrEqual(t, off, len(data))
	}
	require.Equal(t, []string{"ftyp", "moov", "mdat"}, types)
	i := bytes.Index(data, []byte("stco"))
	require.Positive(t, i)
	require.Equal(t, uint32(mdatData), binary.BigEndian.Uint32(data[i+12:]))
	require.Less(t, len(data), 3<<20, "ролик должен быть компактным")
	writeSample(t, "video.mp4", data)
}

// writeSample — SEEDDEMO_MEDIA_OUT=<каталог>: сохранить образцы для ручного просмотра.
func writeSample(t *testing.T, name string, data []byte) {
	dir := os.Getenv("SEEDDEMO_MEDIA_OUT")
	if dir == "" {
		return
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
}
