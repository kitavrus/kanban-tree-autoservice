package seeddemo

// Минимальный кодер MP4/H.264 без внешних утилит: каждый кадр — IDR-срез Baseline-профиля,
// все макроблоки I_PCM (несжатые отсчёты YCbCr 4:2:0, ITU-T H.264 §7.3.5). Сжатия нет —
// поэтому ролики короткие и маленького разрешения (320×192, ~1 МБ на 2 с), зато поток
// валиден для любых декодеров (браузер, QuickTime). Контейнер — ISO BMFF (ftyp + moov +
// mdat, moov впереди — «faststart»), сэмплы в формате AVCC (длина NAL 4 байта).

import (
	"encoding/binary"
	"image"
	"image/color"
)

// bitWriter — запись RBSP побитно (MSB first) с Exp-Golomb.
type bitWriter struct {
	buf []byte
	cur byte
	n   uint8
}

func (b *bitWriter) bit(v uint) {
	b.cur = b.cur<<1 | byte(v&1)
	b.n++
	if b.n == 8 {
		b.buf = append(b.buf, b.cur)
		b.cur, b.n = 0, 0
	}
}

func (b *bitWriter) bits(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		b.bit(uint(v>>uint(i)) & 1)
	}
}

// byteVal — 8 бит; на выровненной позиции — сразу байтом.
func (b *bitWriter) byteVal(v byte) {
	if b.n == 0 {
		b.buf = append(b.buf, v)
		return
	}
	b.bits(uint64(v), 8)
}

func (b *bitWriter) ue(v uint) {
	x := uint64(v) + 1
	n := 0
	for t := x; t > 1; t >>= 1 {
		n++
	}
	b.bits(0, n)
	b.bits(x, n+1)
}

func (b *bitWriter) se(v int) {
	if v > 0 {
		b.ue(uint(2*v - 1))
	} else {
		b.ue(uint(-2 * v))
	}
}

func (b *bitWriter) alignZero() {
	for b.n != 0 {
		b.bit(0)
	}
}

// trailing — rbsp_trailing_bits: стоп-бит 1 и выравнивание нулями.
func (b *bitWriter) trailing() []byte {
	b.bit(1)
	b.alignZero()
	return b.buf
}

// nal — заголовок NAL + RBSP с emulation prevention (00 00 0x → 00 00 03 0x).
func nal(refIdc, typ byte, rbsp []byte) []byte {
	out := make([]byte, 0, len(rbsp)+len(rbsp)/64+1)
	out = append(out, refIdc<<5|typ)
	zeros := 0
	for _, v := range rbsp {
		if zeros >= 2 && v <= 3 {
			out = append(out, 3)
			zeros = 0
		}
		out = append(out, v)
		if v == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
	return out
}

const (
	h264Profile    = 66   // Baseline
	h264Compat     = 0xC0 // constraint_set0/1
	h264Level      = 30   // 3.0
	mp4Timescale   = 600
	log2MaxFrameNo = 4
)

func spsNAL(mbW, mbH int) []byte {
	var b bitWriter
	b.bits(h264Profile, 8)
	b.bits(h264Compat, 8)
	b.bits(h264Level, 8)
	b.ue(0)                  // seq_parameter_set_id
	b.ue(log2MaxFrameNo - 4) // log2_max_frame_num_minus4
	b.ue(2)                  // pic_order_cnt_type: порядок вывода = порядок декодирования
	b.ue(1)                  // max_num_ref_frames
	b.bit(0)                 // gaps_in_frame_num_value_allowed_flag
	b.ue(uint(mbW - 1))      // pic_width_in_mbs_minus1
	b.ue(uint(mbH - 1))      // pic_height_in_map_units_minus1
	b.bit(1)                 // frame_mbs_only_flag
	b.bit(1)                 // direct_8x8_inference_flag
	b.bit(0)                 // frame_cropping_flag
	b.bit(0)                 // vui_parameters_present_flag
	return nal(3, 7, b.trailing())
}

func ppsNAL() []byte {
	var b bitWriter
	b.ue(0)  // pic_parameter_set_id
	b.ue(0)  // seq_parameter_set_id
	b.bit(0) // entropy_coding_mode_flag (CAVLC)
	b.bit(0) // bottom_field_pic_order_in_frame_present_flag
	b.ue(0)  // num_slice_groups_minus1
	b.ue(0)  // num_ref_idx_l0_default_active_minus1
	b.ue(0)  // num_ref_idx_l1_default_active_minus1
	b.bit(0) // weighted_pred_flag
	b.bits(0, 2)
	b.se(0)  // pic_init_qp_minus26
	b.se(0)  // pic_init_qs_minus26
	b.se(0)  // chroma_qp_index_offset
	b.bit(1) // deblocking_filter_control_present_flag
	b.bit(0) // constrained_intra_pred_flag
	b.bit(0) // redundant_pic_cnt_present_flag
	return nal(3, 8, b.trailing())
}

// limited — полный диапазон JFIF → «студийный» (Y 16..235, C 16..240), умолчание H.264 без VUI.
func limitedY(v uint8) byte { return byte(16 + int(v)*219/255) }
func limitedC(v uint8) byte { return byte(16 + int(v)*224/255) }

// idrSliceNAL — IDR-срез из I_PCM-макроблоков.
func idrSliceNAL(img *image.RGBA, idrID int) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	ys := make([]uint8, w*h)
	cbs := make([]uint8, w*h)
	crs := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := img.RGBAAt(x, y)
			yy, cb, cr := color.RGBToYCbCr(p.R, p.G, p.B)
			ys[y*w+x], cbs[y*w+x], crs[y*w+x] = yy, cb, cr
		}
	}
	chroma := func(plane []uint8, x, y int) byte {
		s := int(plane[y*w+x]) + int(plane[y*w+x+1]) + int(plane[(y+1)*w+x]) + int(plane[(y+1)*w+x+1])
		return limitedC(uint8(s / 4))
	}

	var b bitWriter
	b.buf = make([]byte, 0, w*h*3/2+w*h/256*4+16)
	b.ue(0) // first_mb_in_slice
	b.ue(7) // slice_type: I (все срезы кадра)
	b.ue(0) // pic_parameter_set_id
	b.bits(0, log2MaxFrameNo)
	b.ue(uint(idrID)) // idr_pic_id
	b.bit(0)          // no_output_of_prior_pics_flag
	b.bit(0)          // long_term_reference_flag
	b.se(0)           // slice_qp_delta
	b.ue(1)           // disable_deblocking_filter_idc
	for my := 0; my < h/16; my++ {
		for mx := 0; mx < w/16; mx++ {
			b.ue(25) // mb_type I_PCM
			b.alignZero()
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					b.byteVal(limitedY(ys[(my*16+y)*w+mx*16+x]))
				}
			}
			for _, plane := range [][]uint8{cbs, crs} {
				for y := 0; y < 8; y++ {
					for x := 0; x < 8; x++ {
						b.byteVal(chroma(plane, mx*16+x*2, my*16+y*2))
					}
				}
			}
		}
	}
	return nal(3, 5, b.trailing())
}

// --- ISO BMFF ---

func u16(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
func u32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

func box(typ string, payload ...[]byte) []byte {
	size := 8
	for _, p := range payload {
		size += len(p)
	}
	out := make([]byte, 0, size)
	out = append(out, u32(uint32(size))...)
	out = append(out, typ...)
	for _, p := range payload {
		out = append(out, p...)
	}
	return out
}

func fullBox(typ string, ver byte, flags uint32, payload ...[]byte) []byte {
	hdr := u32(uint32(ver)<<24 | flags)
	return box(typ, append([][]byte{hdr}, payload...)...)
}

func join(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

var unityMatrix = join(u32(0x00010000), u32(0), u32(0), u32(0), u32(0x00010000), u32(0), u32(0), u32(0), u32(0x40000000))

// encodeMP4 — кадры одинакового размера (кратного 16) → MP4.
func encodeMP4(frames []*image.RGBA, fps int) []byte {
	w, h := frames[0].Bounds().Dx(), frames[0].Bounds().Dy()
	sps, pps := spsNAL(w/16, h/16), ppsNAL()

	samples := make([][]byte, len(frames))
	for i, f := range frames {
		n := idrSliceNAL(f, i%2)
		samples[i] = join(u32(uint32(len(n))), n)
	}
	delta := uint32(mp4Timescale / fps)
	duration := delta * uint32(len(frames))

	ftyp := box("ftyp", []byte("isom"), u32(0x200), []byte("isomiso2avc1mp41"))

	moov := func(mdatOffset uint32) []byte {
		avcC := box("avcC", []byte{1, sps[1], sps[2], sps[3], 0xFF, 0xE1},
			u16(uint16(len(sps))), sps, []byte{1}, u16(uint16(len(pps))), pps)
		compressor := make([]byte, 32)
		avc1 := box("avc1", make([]byte, 6), u16(1), u16(0), u16(0), make([]byte, 12),
			u16(uint16(w)), u16(uint16(h)), u32(0x00480000), u32(0x00480000), u32(0), u16(1),
			compressor, u16(0x0018), u16(0xFFFF), avcC)
		sizes := make([]byte, 0, 4*len(samples))
		for _, s := range samples {
			sizes = append(sizes, u32(uint32(len(s)))...)
		}
		stbl := box("stbl",
			fullBox("stsd", 0, 0, u32(1), avc1),
			fullBox("stts", 0, 0, u32(1), u32(uint32(len(samples))), u32(delta)),
			fullBox("stsc", 0, 0, u32(1), u32(1), u32(uint32(len(samples))), u32(1)),
			fullBox("stsz", 0, 0, u32(0), u32(uint32(len(samples))), sizes),
			fullBox("stco", 0, 0, u32(1), u32(mdatOffset)),
		)
		minf := box("minf",
			fullBox("vmhd", 0, 1, u16(0), u16(0), u16(0), u16(0)),
			box("dinf", fullBox("dref", 0, 0, u32(1), fullBox("url ", 0, 1))),
			stbl)
		mdia := box("mdia",
			fullBox("mdhd", 0, 0, u32(0), u32(0), u32(mp4Timescale), u32(duration), u16(0x55C4), u16(0)),
			fullBox("hdlr", 0, 0, u32(0), []byte("vide"), make([]byte, 12), []byte("VideoHandler\x00")),
			minf)
		tkhd := fullBox("tkhd", 0, 7, u32(0), u32(0), u32(1), u32(0), u32(duration), make([]byte, 8),
			u16(0), u16(0), u16(0), u16(0), unityMatrix, u32(uint32(w)<<16), u32(uint32(h)<<16))
		mvhd := fullBox("mvhd", 0, 0, u32(0), u32(0), u32(mp4Timescale), u32(duration), u32(0x00010000),
			u16(0x0100), u16(0), make([]byte, 8), unityMatrix, make([]byte, 24), u32(2))
		return box("moov", mvhd, box("trak", tkhd, mdia))
	}

	moovLen := len(moov(0))
	mdatOffset := uint32(len(ftyp) + moovLen + 8)
	return join(ftyp, moov(mdatOffset), box("mdat", samples...))
}
