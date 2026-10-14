package seeddemo

// Демо-вложения заказов (фото/видео приёмки и дефектовки) рисуются программно — без сети и
// внешних утилит: простая векторная сцена (авто спереди с госномером, приборная панель с
// Check Engine, подкапотное пространство, эндоскоп цилиндра, щуп с эмульсией, прогоревшая
// прокладка, расширительный бачок, блок двигателя на стенде) + подписи встроенным
// растровым шрифтом 5×7 (кириллица/латиница/цифры; в stdlib нет шрифтов с кириллицей).
// Фото — JPEG, видео — MP4 (H.264, см. mp4.go).

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------------------
// Растровый шрифт 5×7: '#' — закрашенный пиксель. Кириллица, совпадающая по начертанию с
// латиницей (А В Е К М Н О Р С Т Х), берётся из латинских глыф (glyphAlias).
// ---------------------------------------------------------------------------------------

var glyphs = map[rune][7]string{
	' ': {"     ", "     ", "     ", "     ", "     ", "     ", "     "},
	'0': {" ### ", "#   #", "#  ##", "# # #", "##  #", "#   #", " ### "},
	'1': {"  #  ", " ##  ", "  #  ", "  #  ", "  #  ", "  #  ", " ### "},
	'2': {" ### ", "#   #", "    #", "   # ", "  #  ", " #   ", "#####"},
	'3': {"#####", "   # ", "  #  ", "   # ", "    #", "#   #", " ### "},
	'4': {"   # ", "  ## ", " # # ", "#  # ", "#####", "   # ", "   # "},
	'5': {"#####", "#    ", "#### ", "    #", "    #", "#   #", " ### "},
	'6': {"  ## ", " #   ", "#    ", "#### ", "#   #", "#   #", " ### "},
	'7': {"#####", "    #", "   # ", "  #  ", " #   ", " #   ", " #   "},
	'8': {" ### ", "#   #", "#   #", " ### ", "#   #", "#   #", " ### "},
	'9': {" ### ", "#   #", "#   #", " ####", "    #", "   # ", " ##  "},
	'A': {" ### ", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"},
	'B': {"#### ", "#   #", "#   #", "#### ", "#   #", "#   #", "#### "},
	'C': {" ### ", "#   #", "#    ", "#    ", "#    ", "#   #", " ### "},
	'D': {"#### ", "#   #", "#   #", "#   #", "#   #", "#   #", "#### "},
	'E': {"#####", "#    ", "#    ", "#### ", "#    ", "#    ", "#####"},
	'F': {"#####", "#    ", "#    ", "#### ", "#    ", "#    ", "#    "},
	'G': {" ### ", "#   #", "#    ", "# ###", "#   #", "#   #", " ####"},
	'H': {"#   #", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"},
	'I': {" ### ", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", " ### "},
	'J': {"  ###", "   # ", "   # ", "   # ", "   # ", "#  # ", " ##  "},
	'K': {"#   #", "#  # ", "# #  ", "##   ", "# #  ", "#  # ", "#   #"},
	'L': {"#    ", "#    ", "#    ", "#    ", "#    ", "#    ", "#####"},
	'M': {"#   #", "## ##", "# # #", "# # #", "#   #", "#   #", "#   #"},
	'N': {"#   #", "##  #", "# # #", "#  ##", "#   #", "#   #", "#   #"},
	'O': {" ### ", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
	'P': {"#### ", "#   #", "#   #", "#### ", "#    ", "#    ", "#    "},
	'Q': {" ### ", "#   #", "#   #", "#   #", "# # #", "#  # ", " ## #"},
	'R': {"#### ", "#   #", "#   #", "#### ", "# #  ", "#  # ", "#   #"},
	'S': {" ####", "#    ", "#    ", " ### ", "    #", "    #", "#### "},
	'T': {"#####", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "  #  "},
	'U': {"#   #", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
	'V': {"#   #", "#   #", "#   #", "#   #", "#   #", " # # ", "  #  "},
	'W': {"#   #", "#   #", "#   #", "# # #", "# # #", "# # #", " # # "},
	'X': {"#   #", "#   #", " # # ", "  #  ", " # # ", "#   #", "#   #"},
	'Y': {"#   #", "#   #", " # # ", "  #  ", "  #  ", "  #  ", "  #  "},
	'Z': {"#####", "    #", "   # ", "  #  ", " #   ", "#    ", "#####"},
	'Б': {"#####", "#    ", "#    ", "#### ", "#   #", "#   #", "#### "},
	'Г': {"#####", "#    ", "#    ", "#    ", "#    ", "#    ", "#    "},
	'Д': {" ### ", " #  #", " #  #", " #  #", " #  #", "#####", "#   #"},
	'Ж': {"# # #", "# # #", " ### ", "  #  ", " ### ", "# # #", "# # #"},
	'З': {" ### ", "#   #", "    #", "  ## ", "    #", "#   #", " ### "},
	'И': {"#   #", "#   #", "#  ##", "# # #", "##  #", "#   #", "#   #"},
	'Й': {" # # ", "  #  ", "#   #", "#  ##", "# # #", "##  #", "#   #"},
	'Л': {"  ###", " #  #", " #  #", " #  #", " #  #", "#   #", "#   #"},
	'П': {"#####", "#   #", "#   #", "#   #", "#   #", "#   #", "#   #"},
	'У': {"#   #", "#   #", "#   #", " ####", "    #", "#   #", " ### "},
	'Ф': {"  #  ", " ### ", "# # #", "# # #", "# # #", " ### ", "  #  "},
	'Ц': {"#  # ", "#  # ", "#  # ", "#  # ", "#  # ", "#####", "    #"},
	'Ч': {"#   #", "#   #", "#   #", " ####", "    #", "    #", "    #"},
	'Ш': {"#   #", "#   #", "# # #", "# # #", "# # #", "# # #", "#####"},
	'Щ': {"# # #", "# # #", "# # #", "# # #", "# # #", "#####", "    #"},
	'Ъ': {"##   ", " #   ", " #   ", " ### ", " #  #", " #  #", " ### "},
	'Ы': {"#   #", "#   #", "#   #", "##  #", "# # #", "# # #", "##  #"},
	'Ь': {"#    ", "#    ", "#    ", "#### ", "#   #", "#   #", "#### "},
	'Э': {" ### ", "#   #", "    #", "  ###", "    #", "#   #", " ### "},
	'Ю': {"#  # ", "# # #", "# # #", "### #", "# # #", "# # #", "#  # "},
	'Я': {" ####", "#   #", "#   #", " ####", "  # #", " #  #", "#   #"},
	'Ё': {" # # ", "     ", "#####", "#    ", "#### ", "#    ", "#####"},
	'-': {"     ", "     ", "     ", " ### ", "     ", "     ", "     "},
	'.': {"     ", "     ", "     ", "     ", "     ", "     ", "  #  "},
	',': {"     ", "     ", "     ", "     ", "     ", "  #  ", " #   "},
	':': {"     ", "  #  ", "     ", "     ", "     ", "  #  ", "     "},
	'/': {"    #", "    #", "   # ", "  #  ", " #   ", "#    ", "#    "},
	'(': {"   # ", "  #  ", " #   ", " #   ", " #   ", "  #  ", "   # "},
	')': {" #   ", "  #  ", "   # ", "   # ", "   # ", "  #  ", " #   "},
	'#': {" # # ", "#####", " # # ", " # # ", "#####", " # # ", "     "},
	'·': {"     ", "     ", "     ", "  #  ", "     ", "     ", "     "},
	'+': {"     ", "  #  ", "  #  ", "#####", "  #  ", "  #  ", "     "},
	'%': {"##  #", "##  #", "   # ", "  #  ", " #   ", "#  ##", "#  ##"},
	'!': {"  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "     ", "  #  "},
}

// glyphAlias — кириллица с латинским начертанием.
var glyphAlias = map[rune]rune{
	'А': 'A', 'В': 'B', 'Е': 'E', 'К': 'K', 'М': 'M', 'Н': 'H', 'О': 'O', 'Р': 'P', 'С': 'C', 'Т': 'T', 'Х': 'X',
	'—': '-', '–': '-', '×': 'X', '°': '·',
}

func glyphFor(r rune) ([7]string, bool) {
	r = unicode.ToUpper(r)
	if a, ok := glyphAlias[r]; ok {
		r = a
	}
	g, ok := glyphs[r]
	return g, ok
}

// ---------------------------------------------------------------------------------------
// Холст и примитивы.
// ---------------------------------------------------------------------------------------

type canvas struct {
	img  *image.RGBA
	w, h int
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 255} }

func newCanvas(w, h int, bg color.RGBA) *canvas {
	c := &canvas{img: image.NewRGBA(image.Rect(0, 0, w, h)), w: w, h: h}
	c.rect(0, 0, w, h, bg)
	return c
}

func (c *canvas) set(x, y int, col color.RGBA) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	c.img.SetRGBA(x, y, col)
}

// blend — полупрозрачная точка (пар, блик).
func (c *canvas) blend(x, y int, col color.RGBA, a float64) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	o := c.img.RGBAAt(x, y)
	mix := func(p, q uint8) uint8 { return uint8(float64(p)*(1-a) + float64(q)*a) }
	c.img.SetRGBA(x, y, rgb(mix(o.R, col.R), mix(o.G, col.G), mix(o.B, col.B)))
}

func (c *canvas) rect(x0, y0, x1, y1 int, col color.RGBA) {
	for y := max(y0, 0); y < min(y1, c.h); y++ {
		for x := max(x0, 0); x < min(x1, c.w); x++ {
			c.img.SetRGBA(x, y, col)
		}
	}
}

func (c *canvas) frame(x0, y0, x1, y1, th int, col color.RGBA) {
	c.rect(x0, y0, x1, y0+th, col)
	c.rect(x0, y1-th, x1, y1, col)
	c.rect(x0, y0, x0+th, y1, col)
	c.rect(x1-th, y0, x1, y1, col)
}

func lerp(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }

func (c *canvas) vgrad(x0, y0, x1, y1 int, top, bottom color.RGBA) {
	for y := max(y0, 0); y < min(y1, c.h); y++ {
		t := float64(y-y0) / float64(max(y1-y0, 1))
		col := rgb(lerp(top.R, bottom.R, t), lerp(top.G, bottom.G, t), lerp(top.B, bottom.B, t))
		for x := max(x0, 0); x < min(x1, c.w); x++ {
			c.img.SetRGBA(x, y, col)
		}
	}
}

func (c *canvas) circle(cx, cy, r int, col color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
				c.set(x, y, col)
			}
		}
	}
}

// radial — круг с радиальным градиентом (центр → край).
func (c *canvas) radial(cx, cy, r int, inner, outer color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			d := math.Hypot(float64(x-cx), float64(y-cy)) / float64(r)
			if d <= 1 {
				c.set(x, y, rgb(lerp(inner.R, outer.R, d), lerp(inner.G, outer.G, d), lerp(inner.B, outer.B, d)))
			}
		}
	}
}

func (c *canvas) ring(cx, cy, r, th int, col color.RGBA) {
	ro, ri := r*r, (r-th)*(r-th)
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			d := (x-cx)*(x-cx) + (y-cy)*(y-cy)
			if d <= ro && d >= ri {
				c.set(x, y, col)
			}
		}
	}
}

func (c *canvas) line(x0, y0, x1, y1, th int, col color.RGBA) {
	n := max(abs(x1-x0), abs(y1-y0), 1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		c.circle(x0+int(float64(x1-x0)*t), y0+int(float64(y1-y0)*t), th/2, col)
	}
}

// poly — заливка многоугольника (even-odd по центрам пикселей).
func (c *canvas) poly(pts [][2]int, col color.RGBA) {
	minY, maxY := pts[0][1], pts[0][1]
	for _, p := range pts {
		minY, maxY = min(minY, p[1]), max(maxY, p[1])
	}
	for y := minY; y <= maxY; y++ {
		fy := float64(y) + 0.5
		var xs []float64
		for i := range pts {
			a, b := pts[i], pts[(i+1)%len(pts)]
			ay, by := float64(a[1]), float64(b[1])
			if (ay <= fy && by > fy) || (by <= fy && ay > fy) {
				xs = append(xs, float64(a[0])+(fy-ay)*float64(b[0]-a[0])/(by-ay))
			}
		}
		for i := 0; i+1 < len(xs); i += 2 {
			l, r := min(xs[i], xs[i+1]), max(xs[i], xs[i+1])
			for x := int(math.Ceil(l - 0.5)); float64(x)+0.5 <= r; x++ {
				c.set(x, y, col)
			}
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// text — строка шрифтом 5×7 (масштаб scale, шаг 6·scale); неизвестные символы — пробел.
func (c *canvas) text(x, y int, s string, scale int, col color.RGBA) {
	for _, r := range s {
		g, ok := glyphFor(r)
		if ok {
			for gy, row := range g {
				for gx, px := range row {
					if px == '#' {
						c.rect(x+gx*scale, y+gy*scale, x+(gx+1)*scale, y+(gy+1)*scale, col)
					}
				}
			}
		}
		x += 6 * scale
	}
}

func textWidth(s string, scale int) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return n*6*scale - scale
}

func (c *canvas) textCentered(cx, y int, s string, scale int, col color.RGBA) {
	c.text(cx-textWidth(s, scale)/2, y, s, scale, col)
}

// fit — обрезает строку под ширину width при масштабе scale.
func fit(s string, scale, width int) string {
	r := []rune(s)
	maxN := (width + scale) / (6 * scale)
	if len(r) <= maxN {
		return s
	}
	return string(r[:max(maxN-1, 0)]) + "."
}

// ---------------------------------------------------------------------------------------
// Сцены. Каждая рисует в прямоугольнике r; t ∈ [0,1) — фаза анимации (для фото 0).
// ---------------------------------------------------------------------------------------

type sceneFn func(c *canvas, r image.Rectangle, t float64)

// px — доля ширины/высоты прямоугольника → пиксели.
func px(r image.Rectangle, fx, fy float64) (int, int) {
	return r.Min.X + int(fx*float64(r.Dx())), r.Min.Y + int(fy*float64(r.Dy()))
}

func darker(col color.RGBA, k float64) color.RGBA {
	return rgb(uint8(float64(col.R)*k), uint8(float64(col.G)*k), uint8(float64(col.B)*k))
}

// bodyColor — цвет кузова по названию из specs «Цвет».
func bodyColor(name string) color.RGBA {
	switch strings.ToLower(name) {
	case "белый":
		return rgb(236, 236, 232)
	case "чёрный":
		return rgb(30, 30, 34)
	case "серебристый":
		return rgb(180, 184, 190)
	case "серый":
		return rgb(110, 114, 120)
	case "синий":
		return rgb(40, 70, 150)
	case "красный":
		return rgb(170, 30, 35)
	case "бежевый":
		return rgb(200, 180, 140)
	case "жёлтый":
		return rgb(240, 196, 30)
	}
	return rgb(150, 150, 150)
}

// drawPlate — российский госномер «А123ВС 777»: белое поле, рамка, регион справа с RUS и
// флагом.
func drawPlate(c *canvas, x, y, w, h int, plate string) {
	main, region := plate, ""
	if i := strings.LastIndex(plate, " "); i > 0 {
		main, region = plate[:i], plate[i+1:]
	}
	th := max(2, h/16)
	c.rect(x, y, x+w, y+h, rgb(250, 250, 250))
	c.frame(x, y, x+w, y+h, th, rgb(20, 20, 20))
	regW := w * 26 / 100
	sepX := x + w - regW
	c.rect(sepX, y, sepX+th, y+h, rgb(20, 20, 20))

	scale := max(1, h*62/100/7)
	for textWidth(main, scale) > (sepX-x)-2*th-4 && scale > 1 {
		scale--
	}
	c.textCentered(x+(sepX-x)/2, y+(h-7*scale)/2, main, scale, rgb(15, 15, 15))

	rs := max(1, h*40/100/7)
	for textWidth(region, rs) > regW-2*th-4 && rs > 1 {
		rs--
	}
	c.textCentered(sepX+regW/2, y+th+h/12, region, rs, rgb(15, 15, 15))
	ss := max(1, h*16/100/7)
	ry := y + h - th - h/12 - 7*ss
	rusW := textWidth("RUS", ss)
	flagW := ss * 8
	startX := sepX + regW/2 - (rusW+ss*2+flagW)/2
	c.text(startX, ry, "RUS", ss, rgb(15, 15, 15))
	fx := startX + rusW + ss*2
	fh := 7 * ss / 3
	c.rect(fx, ry, fx+flagW, ry+fh, rgb(255, 255, 255))
	c.frame(fx, ry, fx+flagW, ry+7*ss, 1, rgb(120, 120, 120))
	c.rect(fx, ry+fh, fx+flagW, ry+2*fh, rgb(30, 60, 170))
	c.rect(fx, ry+2*fh, fx+flagW, ry+7*ss, rgb(210, 30, 40))
}

// sceneCar — автомобиль спереди в боксе приёмки, номер на бампере; van — высокий кузов
// (коммерческий / микроавтобус).
func sceneCar(plate, colorName string, van bool) sceneFn {
	body := bodyColor(colorName)
	return func(c *canvas, r image.Rectangle, _ float64) {
		_, floorY := px(r, 0, 0.80)
		c.vgrad(r.Min.X, r.Min.Y, r.Max.X, floorY, rgb(196, 200, 204), rgb(150, 154, 160))
		c.vgrad(r.Min.X, floorY, r.Max.X, r.Max.Y, rgb(92, 94, 98), rgb(60, 62, 66))
		// разметка поста и подъёмник
		for i := 0; i < 2; i++ {
			lx, _ := px(r, 0.12+0.76*float64(i), 0)
			c.rect(lx-6, floorY, lx+6, r.Max.Y, rgb(230, 190, 40))
		}
		cx := r.Min.X + r.Dx()/2
		cw := r.Dx() * 60 / 100
		left, right := cx-cw/2, cx+cw/2
		bodyTop := floorY - r.Dy()*40/100
		cabTop := bodyTop - r.Dy()*22/100
		cabTopW := cw * 62 / 100
		if van {
			bodyTop = floorY - r.Dy()*44/100
			cabTop = bodyTop - r.Dy()*30/100
			cabTopW = cw * 84 / 100
		}
		// тень
		c.rect(left-10, floorY-8, right+10, floorY+10, rgb(40, 40, 44))
		// колёса
		tw := cw * 12 / 100
		c.rect(left+cw*4/100, floorY-r.Dy()*12/100, left+cw*4/100+tw, floorY, rgb(25, 25, 25))
		c.rect(right-cw*4/100-tw, floorY-r.Dy()*12/100, right-cw*4/100, floorY, rgb(25, 25, 25))
		// кабина и стекло
		cabBotW := cw * 88 / 100
		c.poly([][2]int{{cx - cabBotW/2, bodyTop}, {cx + cabBotW/2, bodyTop}, {cx + cabTopW/2, cabTop}, {cx - cabTopW/2, cabTop}}, darker(body, 0.9))
		in := cw * 4 / 100
		c.poly([][2]int{{cx - cabBotW/2 + in, bodyTop - 4}, {cx + cabBotW/2 - in, bodyTop - 4}, {cx + cabTopW/2 - in, cabTop + in}, {cx - cabTopW/2 + in, cabTop + in}}, rgb(70, 92, 112))
		c.poly([][2]int{{cx - cabTopW/4, cabTop + in + 6}, {cx - cabTopW/8, cabTop + in + 6}, {cx - cabBotW/4, bodyTop - 10}, {cx - cabBotW/3, bodyTop - 10}}, rgb(110, 132, 150))
		// зеркала
		c.rect(left-cw*6/100, bodyTop-r.Dy()*6/100, left, bodyTop-r.Dy()*1/100, darker(body, 0.8))
		c.rect(right, bodyTop-r.Dy()*6/100, right+cw*6/100, bodyTop-r.Dy()*1/100, darker(body, 0.8))
		// кузов
		bodyBot := floorY - r.Dy()*8/100
		c.rect(left, bodyTop, right, bodyBot, body)
		c.frame(left, bodyTop, right, bodyBot, 2, darker(body, 0.7))
		// фары, решётка, бампер
		hh := (bodyBot - bodyTop) * 22 / 100
		c.rect(left+cw*3/100, bodyTop+hh/2, left+cw*24/100, bodyTop+hh/2+hh, rgb(245, 240, 210))
		c.rect(right-cw*24/100, bodyTop+hh/2, right-cw*3/100, bodyTop+hh/2+hh, rgb(245, 240, 210))
		c.rect(cx-cw*20/100, bodyTop+hh/2, cx+cw*20/100, bodyTop+hh/2+hh, rgb(35, 35, 38))
		for gx := cx - cw*18/100; gx < cx+cw*18/100; gx += 14 {
			c.rect(gx, bodyTop+hh/2+3, gx+3, bodyTop+hh/2+hh-3, rgb(70, 70, 74))
		}
		bumperTop := bodyTop + (bodyBot-bodyTop)*55/100
		c.rect(left, bumperTop, right, bodyBot, darker(body, 0.82))
		c.rect(left+cw*6/100, bodyBot-(bodyBot-bumperTop)/4, right-cw*6/100, bodyBot-(bodyBot-bumperTop)/8, rgb(30, 30, 30))
		if plate != "" {
			pw := cw * 40 / 100
			ph := pw * 22 / 100
			drawPlate(c, cx-pw/2, bumperTop+(bodyBot-bumperTop)/2-ph/2-ph/4, pw, ph, plate)
		}
	}
}

// sceneDash — приборная панель: тахометр/спидометр, значок Check Engine, коды ошибок и
// пробег.
func sceneDash(codes, mileage string) sceneFn {
	return func(c *canvas, r image.Rectangle, t float64) {
		c.vgrad(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, rgb(28, 28, 32), rgb(10, 10, 12))
		rad := r.Dy() * 36 / 100
		for i, fx := range []float64{0.22, 0.78} {
			gx, gy := px(r, fx, 0.48)
			c.radial(gx, gy, rad, rgb(40, 40, 46), rgb(18, 18, 20))
			c.ring(gx, gy, rad, 5, rgb(200, 200, 205))
			for k := 0; k <= 10; k++ {
				a := math.Pi*0.75 + float64(k)*math.Pi*1.5/10
				x0, y0 := gx+int(math.Cos(a)*float64(rad-10)), gy+int(math.Sin(a)*float64(rad-10))
				x1, y1 := gx+int(math.Cos(a)*float64(rad-28)), gy+int(math.Sin(a)*float64(rad-28))
				col := rgb(220, 220, 225)
				if i == 0 && k >= 8 {
					col = rgb(220, 50, 40)
				}
				c.line(x0, y0, x1, y1, 4, col)
			}
			na := math.Pi*0.75 + math.Pi*1.5*0.12
			if i == 0 {
				na = math.Pi*0.75 + math.Pi*1.5*(0.09+0.02*math.Sin(t*2*math.Pi*3))
			}
			c.line(gx, gy, gx+int(math.Cos(na)*float64(rad-30)), gy+int(math.Sin(na)*float64(rad-30)), 5, rgb(240, 90, 30))
			c.circle(gx, gy, 10, rgb(60, 60, 66))
		}
		label := []string{"X1000", "KM/H"}
		for i, fx := range []float64{0.22, 0.78} {
			gx, gy := px(r, fx, 0.48)
			c.textCentered(gx, gy+rad/3, label[i], 2, rgb(170, 170, 175))
		}
		// Check Engine (мигает в видео)
		cx, cy := px(r, 0.5, 0.30)
		if t < 0.5 {
			amber := rgb(255, 170, 20)
			s := r.Dy() / 60
			c.rect(cx-9*s, cy-4*s, cx+7*s, cy+6*s, amber)
			c.rect(cx-5*s, cy-7*s, cx+3*s, cy-4*s, amber)
			c.rect(cx-12*s, cy-2*s, cx-9*s, cy+4*s, amber)
			c.rect(cx+7*s, cy-1*s, cx+11*s, cy+3*s, amber)
			c.rect(cx-6*s, cy-2*s, cx+4*s, cy+4*s, rgb(28, 28, 32))
			c.textCentered(cx, cy+9*s, "CHECK", 3, amber)
		}
		c.textCentered(cx, r.Min.Y+r.Dy()*62/100, codes, 4, rgb(255, 170, 20))
		ox, oy := px(r, 0.5, 0.82)
		w := textWidth(mileage, 3) + 24
		c.rect(ox-w/2, oy-8, ox+w/2, oy+21+8, rgb(5, 25, 30))
		c.frame(ox-w/2, oy-8, ox+w/2, oy+21+8, 2, rgb(60, 90, 100))
		c.textCentered(ox, oy, mileage, 3, rgb(120, 230, 240))
	}
}

// markRing — красное кольцо-выноска «смотри сюда».
func markRing(c *canvas, x, y, rad int, pulse float64) {
	rr := rad + int(pulse*float64(rad)/4)
	c.ring(x, y, rr, 6, rgb(230, 30, 30))
	c.line(x+rr*7/10, y-rr*7/10, x+rr*7/10+rad, y-rr*7/10-rad, 6, rgb(230, 30, 30))
}

// sceneEngine — подкапотное пространство: клапанная крышка с катушками, впуск, патрубки;
// mark — точка дефекта (доли), shake — тряска двигателя в видео, steam — пар/дым.
func sceneEngine(markX, markY float64, shake, steam bool) sceneFn {
	return func(c *canvas, r image.Rectangle, t float64) {
		c.vgrad(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, rgb(48, 48, 52), rgb(22, 22, 24))
		dx, dy := 0, 0
		if shake {
			dx = int(5 * math.Sin(t*2*math.Pi*5))
			dy = int(4 * math.Cos(t*2*math.Pi*7))
		}
		// радиатор сверху
		rx0, ry0 := px(r, 0.08, 0.04)
		rx1, ry1 := px(r, 0.92, 0.16)
		c.rect(rx0, ry0, rx1, ry1, rgb(30, 30, 32))
		for x := rx0; x < rx1; x += 8 {
			c.rect(x, ry0, x+2, ry1, rgb(60, 60, 64))
		}
		// блок/крышка
		x0, y0 := px(r, 0.20, 0.26)
		x1, y1 := px(r, 0.80, 0.70)
		x0, x1, y0, y1 = x0+dx, x1+dx, y0+dy, y1+dy
		c.rect(x0-10, y1-20, x1+10, y1+r.Dy()/8, rgb(70, 70, 74))
		c.rect(x0, y0, x1, y1, rgb(120, 122, 128))
		c.frame(x0, y0, x1, y1, 4, rgb(80, 82, 86))
		c.vgrad(x0+8, y0+8, x1-8, y0+(y1-y0)/3, rgb(150, 152, 158), rgb(120, 122, 128))
		cw := (x1 - x0) / 4
		for i := 0; i < 4; i++ {
			kx := x0 + cw*i + cw/2
			c.rect(kx-cw/5, y0+(y1-y0)/4, kx+cw/5, y0+(y1-y0)*3/4, rgb(25, 25, 28))
			c.circle(kx, y0+(y1-y0)/4, cw/6, rgb(40, 40, 44))
		}
		// маслозаливная крышка
		c.circle(x1-cw/2, y0+18, 14, rgb(230, 190, 40))
		// патрубки
		hx, hy := px(r, 0.10, 0.20)
		c.line(hx, hy, x0+dx, y0+(y1-y0)/2, 14, rgb(15, 15, 15))
		hx2, hy2 := px(r, 0.92, 0.85)
		c.line(hx2, hy2, x1, y1-10, 14, rgb(15, 15, 15))
		// расширительный бачок
		tx, ty := px(r, 0.84, 0.30)
		c.rect(tx, ty, tx+r.Dx()/14, ty+r.Dy()/5, rgb(215, 215, 205))
		c.rect(tx, ty+r.Dy()/10, tx+r.Dx()/14, ty+r.Dy()/5, rgb(230, 120, 150))
		if steam {
			for i := 0; i < 160; i++ {
				fx := 0.3 + 0.4*math.Mod(float64(i)*0.6180339, 1)
				fy := 0.1 + 0.5*math.Mod(float64(i)*0.4142135+t, 1)
				sx, sy := px(r, fx, fy)
				for k := 0; k < 14; k++ {
					for j := 0; j < 14; j++ {
						c.blend(sx+k, sy+j, rgb(235, 235, 235), 0.08)
					}
				}
			}
		}
		if markX >= 0 {
			mx, my := px(r, markX, markY)
			pulse := 0.0
			if shake {
				pulse = math.Abs(math.Sin(t * 2 * math.Pi * 2))
			}
			markRing(c, mx+dx, my+dy, r.Dy()/10, pulse)
		}
	}
}

// sceneEndoscope — кадр эндоскопа: стенка цилиндра, вертикальные задиры.
func sceneEndoscope(label string) sceneFn {
	return func(c *canvas, r image.Rectangle, _ float64) {
		c.rect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, rgb(0, 0, 0))
		cx, cy := px(r, 0.5, 0.5)
		rad := r.Dy() * 46 / 100
		c.radial(cx, cy, rad, rgb(190, 184, 170), rgb(30, 28, 26))
		c.radial(cx, cy, rad/4, rgb(20, 20, 20), rgb(60, 58, 54))
		// хон (сетка) и задиры
		for k := -6; k <= 6; k++ {
			c.line(cx+k*rad/8-rad/3, cy-rad*2/3, cx+k*rad/8+rad/3, cy+rad*2/3, 1, rgb(150, 145, 135))
		}
		for i, off := range []int{-rad / 3, -rad / 5, -rad / 12, rad / 6} {
			c.line(cx+off, cy-rad*3/4+i*8, cx+off+4, cy+rad*3/5, 4+i%2*2, rgb(110, 60, 35))
		}
		markRing(c, cx-rad/6, cy+rad/8, rad/3, 0)
		c.text(r.Min.X+20, r.Min.Y+20, label, 4, rgb(80, 255, 120))
		c.text(r.Max.X-textWidth("REC", 3)-20, r.Min.Y+20, "REC", 3, rgb(255, 60, 60))
	}
}

// sceneDipstick — масляный щуп с эмульсией («кофе с молоком»).
func sceneDipstick() sceneFn {
	return func(c *canvas, r image.Rectangle, _ float64) {
		c.vgrad(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, rgb(215, 215, 210), rgb(170, 170, 165))
		// ветошь
		gx, gy := px(r, 0.05, 0.62)
		c.poly([][2]int{{gx, gy}, {gx + r.Dx()/3, gy - 20}, {gx + r.Dx()/3 + 30, r.Max.Y - 10}, {gx + 10, r.Max.Y - 30}}, rgb(170, 40, 40))
		x0, y0 := px(r, 0.10, 0.80)
		x1, y1 := px(r, 0.86, 0.26)
		c.line(x0, y0, x1, y1, 12, rgb(190, 192, 196))
		c.circle(x0, y0, 34, rgb(240, 200, 30))
		c.circle(x0, y0, 16, rgb(215, 215, 210))
		// метки MIN/MAX
		for i, f := range []float64{0.78, 0.88} {
			mx := x0 + int(float64(x1-x0)*f)
			my := y0 + int(float64(y1-y0)*f)
			c.line(mx-10, my-14, mx+10, my+14, 4, rgb(60, 60, 60))
			c.text(mx-30, my+26, []string{"MIN", "MAX"}[i], 3, rgb(40, 40, 40))
		}
		// эмульсия
		for i := 0; i < 26; i++ {
			f := 0.74 + 0.22*float64(i)/26
			mx := x0 + int(float64(x1-x0)*f)
			my := y0 + int(float64(y1-y0)*f)
			c.circle(mx+(i%3-1)*5, my+(i%4-2)*3, 12+i%3*3, rgb(205, 175, 125))
		}
		mx, my := px(r, 0.80, 0.33)
		markRing(c, mx, my, r.Dy()/7, 0)
	}
}

// sceneGasket — снятая прокладка ГБЦ на верстаке с прогаром между цилиндрами.
func sceneGasket(burnBetween int) sceneFn {
	return func(c *canvas, r image.Rectangle, _ float64) {
		c.rect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, rgb(150, 105, 62))
		for y := r.Min.Y; y < r.Max.Y; y += 23 {
			c.rect(r.Min.X, y, r.Max.X, y+3, rgb(135, 92, 52))
		}
		x0, y0 := px(r, 0.08, 0.28)
		x1, y1 := px(r, 0.92, 0.72)
		c.rect(x0, y0, x1, y1, rgb(178, 178, 184))
		c.frame(x0, y0, x1, y1, 3, rgb(120, 120, 126))
		bw := (x1 - x0) / 4
		rad := min(bw, y1-y0) * 40 / 100
		for i := 0; i < 4; i++ {
			bx := x0 + bw*i + bw/2
			by := (y0 + y1) / 2
			c.ring(bx, by, rad+10, 8, rgb(110, 110, 116))
			c.circle(bx, by, rad, rgb(150, 105, 62))
			c.circle(bx-bw/2+8, y0+16, 7, rgb(150, 105, 62))
			c.circle(bx-bw/2+8, y1-16, 7, rgb(150, 105, 62))
		}
		bx := x0 + bw*burnBetween
		by := (y0 + y1) / 2
		c.rect(bx-14, by-rad/2, bx+14, by+rad/2, rgb(45, 32, 26))
		c.rect(bx-6, by-rad/2-10, bx+6, by+rad/2+10, rgb(25, 18, 14))
		markRing(c, bx, by, rad*8/10, 0)
	}
}

// sceneTank — расширительный бачок: уровень и пузыри выхлопных газов (в видео движутся).
func sceneTank() sceneFn {
	return func(c *canvas, r image.Rectangle, t float64) {
		c.vgrad(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, rgb(52, 52, 56), rgb(25, 25, 27))
		x0, y0 := px(r, 0.30, 0.18)
		x1, y1 := px(r, 0.70, 0.92)
		c.rect(x0, y0, x1, y1, rgb(222, 222, 212))
		level := y0 + (y1-y0)*45/100
		c.rect(x0+6, level, x1-6, y1-6, rgb(232, 118, 150))
		c.frame(x0, y0, x1, y1, 5, rgb(150, 150, 140))
		cx := (x0 + x1) / 2
		c.rect(cx-50, y0-50, cx+50, y0, rgb(20, 20, 22))
		for i, lab := range []string{"MAX", "MIN"} {
			ly := y0 + (y1-y0)*(35+i*30)/100
			c.rect(x1-50, ly, x1-6, ly+4, rgb(60, 60, 60))
			c.text(x1+12, ly-10, lab, 3, rgb(220, 220, 220))
		}
		for i := 0; i < 12; i++ {
			ph := math.Mod(t+float64(i)/12, 1)
			bx := x0 + 30 + (i*53)%(x1-x0-60)
			by := y1 - 20 - int(ph*float64(y1-20-level))
			rad := 6 + i%4*3
			c.circle(bx, by, rad, rgb(250, 190, 205))
			c.ring(bx, by, rad, 2, rgb(255, 240, 245))
		}
		// пена у поверхности
		for x := x0 + 10; x < x1-10; x += 16 {
			c.circle(x, level, 7, rgb(250, 215, 225))
		}
	}
}

// sceneBlock — блок цилиндров на стенде (вид сверху на привалочную плоскость).
func sceneBlock(markCyl int) sceneFn {
	return func(c *canvas, r image.Rectangle, _ float64) {
		c.vgrad(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, rgb(120, 124, 128), rgb(80, 82, 86))
		// стенд
		sx0, sy0 := px(r, 0.20, 0.80)
		sx1, _ := px(r, 0.80, 0.80)
		c.line(sx0, sy0, sx0-40, r.Max.Y, 18, rgb(30, 70, 160))
		c.line(sx1, sy0, sx1+40, r.Max.Y, 18, rgb(30, 70, 160))
		x0, y0 := px(r, 0.10, 0.20)
		x1, y1 := px(r, 0.90, 0.78)
		c.rect(x0, y0, x1, y1, rgb(168, 170, 176))
		c.frame(x0, y0, x1, y1, 4, rgb(110, 112, 118))
		bw := (x1 - x0) / 4
		rad := min(bw, y1-y0) * 40 / 100
		for i := 0; i < 4; i++ {
			bx := x0 + bw*i + bw/2
			by := (y0 + y1) / 2
			c.radial(bx, by, rad, rgb(70, 70, 74), rgb(150, 150, 156))
			c.ring(bx, by, rad, 4, rgb(205, 205, 210))
			for _, k := range [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
				c.circle(bx+k[0]*(bw/2-10), by+k[1]*(rad+8), 6, rgb(60, 60, 64))
			}
			if i+1 == markCyl {
				for j := -2; j <= 2; j++ {
					c.line(bx+j*rad/5, by-rad*2/3, bx+j*rad/5+3, by+rad*2/3, 3, rgb(120, 70, 40))
				}
				markRing(c, bx, by, rad+16, 0)
			}
		}
		c.textCentered((x0+x1)/2, y0+14, "1   2   3   4", 3, rgb(60, 60, 64))
	}
}

// ---------------------------------------------------------------------------------------
// Сборка кадра: шапка (заказ / госномер), сцена, подпись снизу.
// ---------------------------------------------------------------------------------------

const (
	photoW, photoH = 960, 640
	// Видео рисуется 960×576 и уменьшается в 3 раза → 320×192 (кратно 16 для H.264).
	videoW, videoH, videoDown = 960, 576, 3
)

// composeFrame — кадр w×h: шапка с header, сцена, подпись footer; rec — индикатор записи.
func composeFrame(w, h int, header, footer string, sc sceneFn, t float64, video bool) *canvas {
	c := newCanvas(w, h, rgb(0, 0, 0))
	hs, fs := 4, 3
	barH, footH := 60, 60
	if video {
		hs, fs = 6, 5
		barH, footH = 72, 72
	}
	sc(c, image.Rect(0, barH, w, h-footH), t)
	c.rect(0, 0, w, barH, rgb(20, 24, 30))
	c.text(20, (barH-7*hs)/2, fit(header, hs, w-40-90), hs, rgb(255, 255, 255))
	if video {
		if t < 0.5 {
			c.circle(w-48, barH/2, 16, rgb(240, 40, 40))
		}
	} else {
		c.text(w-20-textWidth("ФОТО", 3), (barH-21)/2, "ФОТО", 3, rgb(140, 200, 255))
	}
	c.rect(0, h-footH, w, h, rgb(20, 24, 30))
	c.text(20, h-footH+(footH-7*fs)/2, fit(footer, fs, w-40), fs, rgb(255, 210, 60))
	return c
}

// renderPhoto — JPEG 960×640.
func renderPhoto(header, footer string, sc sceneFn) ([]byte, error) {
	c := composeFrame(photoW, photoH, header, footer, sc, 0, false)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, c.img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// downscale — усреднение блоков k×k.
func downscale(src *image.RGBA, k int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx()/k, b.Dy()/k
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, bl int
			for j := 0; j < k; j++ {
				for i := 0; i < k; i++ {
					p := src.RGBAAt(x*k+i, y*k+j)
					r, g, bl = r+int(p.R), g+int(p.G), bl+int(p.B)
				}
			}
			n := k * k
			dst.SetRGBA(x, y, rgb(uint8(r/n), uint8(g/n), uint8(bl/n)))
		}
	}
	return dst
}

// videoFrames / videoFPS — короткий ролик 2 с.
const (
	videoFrames = 12
	videoFPS    = 6
)

// renderVideo — MP4 320×192, videoFrames кадров (см. mp4.go).
func renderVideo(header, footer string, sc sceneFn) []byte {
	frames := make([]*image.RGBA, 0, videoFrames)
	for i := 0; i < videoFrames; i++ {
		t := float64(i) / videoFrames
		c := composeFrame(videoW, videoH, header, footer, sc, t, true)
		frames = append(frames, downscale(c.img, videoDown))
	}
	return encodeMP4(frames, videoFPS)
}
