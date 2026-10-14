package seeddemo

import "github.com/kitavrus/kanban-tree/internal/features/board"

// Заказы автосервиса по колонкам доски (колонки сеет миграция 0005: «Новые» = приёмка,
// «Согласование» — смета у клиента, «В работе», «Ждёт запчасти», «Готово» = к выдаче/выдан).

// demoSubtaskItem — услуга заказа: service — code из demoServices; assignee -1 — без
// исполнителя; dueOffset — дни от сегодня; goodsOverride/priceOverride — вместо комплекта
// и цены услуги. payout/payoutPaid — выплата мастеру (executor-payouts, D71).
type demoSubtaskItem struct {
	service       string
	status        string
	assignee      int
	dueOffset     *int
	priceOverride string
	goodsOverride []protoGood
	logs          []demoLog
	payout        string
	payoutPaid    bool
}

// demoLog — запись журнала работ (subtask_logs). user — индекс demoExecutors, -1 — админ.
type demoLog struct {
	user     int
	hoursAgo int
	text     string
}

// demoTaskEquipment — автомобиль заказа: комплектность/состояние на приёмке (акты).
type demoTaskEquipment struct {
	key                     string
	completeness, condition string
}

// Сцены вложений (media.go).
const (
	mediaCar       = "car"       // авто спереди с госномером (для агрегата — блок на стенде)
	mediaDash      = "dash"      // приборная панель, Check Engine, коды, пробег
	mediaEngine    = "engine"    // подкапотное, выноска на дефект
	mediaEndoscope = "endoscope" // кадр эндоскопа
	mediaDipstick  = "dipstick"  // щуп с эмульсией
	mediaGasket    = "gasket"    // прогоревшая прокладка ГБЦ
	mediaTank      = "tank"      // расширительный бачок с пузырями
	mediaBlock     = "block"     // блок цилиндров на стенде
)

// demoMedia — вложение заказа: фото (JPEG) или видео (MP4). name — исходное имя файла без
// расширения; caption — подпись на кадре; codes/mileage — для приборной панели; markX/markY
// — выноска на подкапотном фото (доли кадра, -1 — без неё).
type demoMedia struct {
	video        bool
	scene        string
	name         string
	caption      string
	user         int
	hoursAgo     int
	codes        string
	markX, markY float64
	shake, steam bool
}

// demoTaskItem — заказ доски.
type demoTaskItem struct {
	column    string
	clientIdx int
	title     string
	assignee  int
	dueOffset *int
	paid      bool
	descHTML  string
	equipment []demoTaskEquipment
	subtasks  []demoSubtaskItem
	media     []demoMedia
}

func offset(n int) *int { return &n }

const (
	stNew  = board.SubtaskStatusNew
	stWork = board.SubtaskStatusInProgress
	stDone = board.SubtaskStatusDone
)

// intakePhoto — фото приёмки (снимает мастер-приёмщик).
func intakePhoto(hoursAgo int) demoMedia {
	return demoMedia{scene: mediaCar, name: "Фото приёмки", caption: "ФОТО ПРИЁМКИ", user: exIntake, hoursAgo: hoursAgo}
}

var demoTasks = []demoTaskItem{
	// --- Новые (приёмка) ---
	{
		column: "Новые", clientIdx: 1, title: "Hyundai Solaris О718МР 199: троит на холодную, Check Engine P0300", assignee: exDiag, dueOffset: offset(1),
		descHTML: "<p>Со слов клиента: первые 3–5 минут после холодного пуска двигатель троит, на прогретом — ровно. Горит Check Engine.</p>" +
			"<ul><li>Пробег 187 000 км, свечи меняли ~40 000 км назад</li><li>Ошибки (сканер на заправке): P0300, P0302</li><li>Проверить свечи, катушки, компрессию 2-го цилиндра</li></ul>",
		equipment: []demoTaskEquipment{{key: "solaris", completeness: "Ключ, СТС", condition: "Скол на лобовом стекле, царапина на заднем бампере. Топливо ~1/4 бака"}},
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stNew, assignee: exDiag, dueOffset: offset(1), logs: []demoLog{
				{exIntake, 20, "Пробег 187 000, на приёмке горит CE, клиент жалуется на троение на холодную. Фото приборки и видео работы на ХХ приложены."},
			}},
			{service: "compr", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{
			intakePhoto(20),
			{scene: mediaDash, name: "Приборная панель на приёмке", caption: "ГОРИТ CHECK ENGINE НА ПРИЁМКЕ", codes: "P0300  P0302", user: exIntake, hoursAgo: 20},
			{video: true, scene: mediaEngine, name: "Видео - троение на ХХ", caption: "ТРОИТ НА ХОЛОДНУЮ", markX: 0.42, markY: 0.45, shake: true, user: exIntake, hoursAgo: 20},
		},
	},
	{
		column: "Новые", clientIdx: 9, title: "Mazda CX-5 Е019ТТ 178: ТО-120 000 (масло, фильтр, свечи, дроссель)", assignee: exMotor, dueOffset: offset(3),
		equipment: []demoTaskEquipment{{key: "cx5", completeness: "Ключ, сервисная книжка"}},
		subtasks: []demoSubtaskItem{
			{service: "oil", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("OIL-5W30-208", "4.2"), g("PE01-14-302A", "1")}, logs: []demoLog{
				{exIntake, 6, "Клиент записан на 10:00, ждёт в зоне отдыха. Масло 5W-30 согласовано."},
			}},
			{service: "plugs", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("SK16HR11", "4")}},
			{service: "throttle", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{intakePhoto(6)},
	},
	{
		column: "Новые", clientIdx: 6, title: "ГАЗель Next В734СА 750: тяжело заводится на холодную, белый дым", assignee: exNone, dueOffset: offset(2),
		descHTML:  "<p>Пригнали на буксире. Заводится с 5–6 попытки, первые минуты белый дым и вибрация, тяга слабая.</p><p>Подозрение: свечи накала / форсунки / компрессия.</p>",
		equipment: []demoTaskEquipment{{key: "gazelle", completeness: "Ключ, путевой лист", condition: "Вмятина на правой сдвижной двери"}},
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stNew, assignee: exNone, logs: []demoLog{
				{exIntake, 3, "Машина нужна клиенту к пятнице — развозка по объектам. Разрешили диагностику до 5 000 ₽ без согласования."},
			}},
			{service: "compr", status: stNew, assignee: exNone},
		},
		media: []demoMedia{
			intakePhoto(3),
			{scene: mediaEngine, name: "Подкапотное - копоть на впуске", caption: "КОПОТЬ НА ВПУСКЕ, ПОДТЁК ДИЗТОПЛИВА", markX: 0.30, markY: 0.55, user: exIntake, hoursAgo: 3},
		},
	},
	// --- Согласование ---
	{
		column: "Согласование", clientIdx: 2, title: "VW Polo К902ТН 750: стук в двигателе, расход масла 1 л на 1000 км", assignee: exMotor,
		descHTML: "<p>Стук на прогретом двигателе под нагрузкой, сизый дым при перегазовке, расход масла ~1 л/1000 км.</p>" +
			"<p><strong>Дефектовка:</strong> компрессия 1–2–3–4: 12,5 / 12,0 / 8,5 / 12,0 бар; эндоскопия — задиры в 3-м цилиндре. Рекомендован капремонт.</p>",
		equipment: []demoTaskEquipment{{key: "polo", completeness: "Ключ", condition: "Масляный налёт на моторном щите"}},
		subtasks: []demoSubtaskItem{
			{service: "noise", status: stDone, assignee: exDiag, payout: "600", logs: []demoLog{
				{exDiag, 50, "Стук глухой, усиливается при прогазовке на 2500–3000 об/мин. Похоже на поршневую группу 3-го цилиндра."},
			}},
			{service: "compr", status: stDone, assignee: exMotor, logs: []demoLog{
				{exMotor, 48, "Компрессия 1-2-3-4: 12,5 / 12,0 / 8,5 / 12,0 бар. С маслом в 3-м поднялась до 10,5 — кольца."},
			}},
			{service: "endo", status: stDone, assignee: exDiag, logs: []demoLog{
				{exDiag, 47, "Эндоскоп: в 3-м цилиндре вертикальные задиры на стенке, хон стёрт. Фото приложено."},
			}},
			// запчастей VAG под капремонт на складе нет — комплект после согласования
			{service: "overhaul", status: stNew, assignee: exMotor, priceOverride: "95000", goodsOverride: []protoGood{}, logs: []demoLog{
				{exAdmin, 30, "Смета на капремонт 95 000 ₽ + запчасти по факту отправлена клиенту в WhatsApp, ждём решения."},
			}},
			{service: "engineRR", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{
			intakePhoto(52),
			{scene: mediaEndoscope, name: "Эндоскопия - цилиндр 3", caption: "ЦИЛ. 3 — ВЕРТИКАЛЬНЫЕ ЗАДИРЫ", user: exDiag, hoursAgo: 47},
		},
	},
	{
		column: "Согласование", clientIdx: 7, title: "Fiat Ducato У603МН 190: течь масла, замена ремня ГРМ по регламенту", assignee: exMotor,
		descHTML:  "<p>Пробег 402 600 км, ремень ГРМ меняли 120 000 км назад — пора по регламенту. Течь масла из-под шкива коленвала, пятно на стоянке.</p>",
		equipment: []demoTaskEquipment{{key: "ducato", completeness: "Ключ, СТС", condition: "Масляные следы под двигателем"}},
		subtasks: []demoSubtaskItem{
			{service: "noise", status: stDone, assignee: exMotor, payout: "500", payoutPaid: true, logs: []demoLog{
				{exMotor, 26, "Течь по переднему сальнику коленвала, ремень ГРМ с трещинами на тыльной стороне — менять обязательно."},
			}},
			{service: "timing", status: stNew, assignee: exMotor, priceOverride: "9500", goodsOverride: []protoGood{g("KTB553", "1")}},
			{service: "seals", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("12019350B", "1")}, logs: []demoLog{
				{exAdmin, 20, "Клиент согласовывает ГРМ + сальник, по деньгам думает до завтра."},
			}},
		},
		media: []demoMedia{
			intakePhoto(28),
			{scene: mediaEngine, name: "Течь по сальнику коленвала", caption: "ТЕЧЬ ПО ПЕРЕДНЕМУ САЛЬНИКУ КОЛЕНВАЛА", markX: 0.25, markY: 0.75, user: exMotor, hoursAgo: 26},
		},
	},
	// --- В работе ---
	{
		column: "В работе", clientIdx: 5, title: "Skoda Octavia Т389ХК 799 (такси): плавают обороты ХХ + ремень ГРМ по пробегу", assignee: exMotor, dueOffset: offset(0),
		equipment: []demoTaskEquipment{{key: "taxi-octavia", completeness: "Ключ, путевой лист", condition: "Потёртости салона, кузов без повреждений"}},
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exDiag, payout: "700", payoutPaid: true, logs: []demoLog{
				{exDiag, 28, "Ошибки P0507 (обороты ХХ выше нормы), P2187 (бедная смесь на ХХ). Похоже на подсос воздуха."},
			}},
			{service: "smoke", status: stDone, assignee: exDiag, payout: "700", logs: []demoLog{
				{exDiag, 26, "Дымогенератор: трещина патрубка вентиляции картерных газов у клапанной крышки. Патрубок таксопарк купил сам."},
			}},
			{service: "timing", status: stWork, assignee: exMotor, dueOffset: offset(0), goodsOverride: []protoGood{g("CT1168K1", "1")}, logs: []demoLog{
				{exMotor, 4, "Сняли старый ремень — износ по кромке, ролик с люфтом. Ставим новый комплект, метки совпали."},
			}},
		},
		media: []demoMedia{
			intakePhoto(30),
			{scene: mediaEngine, name: "Дымогенератор - подсос", caption: "ТРЕЩИНА ПАТРУБКА ВКГ (ДЫМ)", markX: 0.55, markY: 0.30, steam: true, user: exDiag, hoursAgo: 26},
		},
	},
	{
		column: "В работе", clientIdx: 0, title: "Lada Vesta Е245КХ 797: перегрев, уходит антифриз — пробита прокладка ГБЦ", assignee: exMotor, dueOffset: offset(-1),
		descHTML: "<p>Перегрев в пробке, стрелка в красной зоне, пар из-под капота. Антифриз уходит ~0,5 л в день, внешних течей нет.</p>" +
			"<ul><li>Тест CO₂ — положительный, пузыри в расширительном бачке</li><li>ГБЦ — в мехцех на опрессовку и проверку плоскости</li></ul>",
		equipment: []demoTaskEquipment{{key: "vesta", completeness: "Ключ, СТС", condition: "Привезли на эвакуаторе, антифриз ниже MIN"}},
		subtasks: []demoSubtaskItem{
			{service: "cooltest", status: stDone, assignee: exDiag, payout: "900", payoutPaid: true, logs: []demoLog{
				{exDiag, 46, "Опрессовка 1,4 бар — давление падает за 10 минут, внешних течей нет. Тест CO₂ в бачке пожелтел — выхлоп в системе охлаждения."},
			}},
			{service: "headgasket", status: stWork, assignee: exMotor, dueOffset: offset(-1), logs: []demoLog{
				{exMotor, 22, "ГБЦ снята. Прокладка прогорела между 2-м и 3-м цилиндром, фото приложено. Головка ушла в мехцех."},
			}},
			{service: "headmill", status: stWork, assignee: exNone, logs: []demoLog{
				{exAdmin, 20, "ГБЦ отдана в мехцех «ГБЦ-Сервис», готовность завтра к 12:00."},
			}},
			{service: "coolant", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{
			intakePhoto(48),
			{video: true, scene: mediaTank, name: "Видео - пузыри в расширительном бачке", caption: "ПУЗЫРИ В БАЧКЕ", user: exDiag, hoursAgo: 46},
			{scene: mediaGasket, name: "Снятая прокладка ГБЦ", caption: "ПРОГАР ПРОКЛАДКИ МЕЖДУ 2 И 3 ЦИЛИНДРОМ", user: exMotor, hoursAgo: 22},
		},
	},
	{
		column: "В работе", clientIdx: 8, title: "Toyota LC Prado С808ВО 77: ТО-165 000, топливный фильтр, рывки на разгоне", assignee: exMotor, dueOffset: offset(1),
		equipment: []demoTaskEquipment{{key: "prado", completeness: "Ключ, сервисная книжка"}},
		subtasks: []demoSubtaskItem{
			{service: "oil", status: stDone, assignee: exMotor, payout: "900", goodsOverride: []protoGood{g("OIL-5W30-208", "7.5"), g("04152-YZZA6", "1")}, logs: []demoLog{
				{exMotor, 5, "Масло слито, на магнитной пробке чисто. Залито 7,5 л 5W-30."},
			}},
			{service: "fuel", status: stWork, assignee: exMotor},
			{service: "diag", status: stNew, assignee: exElectric, logs: []demoLog{
				{exIntake, 26, "Клиент: на разгоне 1500–2000 об/мин лёгкие рывки, началось после заправки на трассе. Взять пробу топлива."},
			}},
		},
		media: []demoMedia{intakePhoto(26)},
	},
	{
		column: "В работе", clientIdx: 4, title: "Nissan Qashqai Н481ОР 50: плавают обороты, ошибка P0101 (ДМРВ)", assignee: exElectric, dueOffset: offset(1),
		equipment: []demoTaskEquipment{{key: "qashqai", completeness: "Ключ"}},
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exElectric, payout: "800", payoutPaid: true, logs: []demoLog{
				{exElectric, 30, "P0101 — ДМРВ вне диапазона. На ХХ 2,1 г/с при норме 2,6–3,0; на прогазовке не растёт. Разъём чистый."},
			}},
			{service: "maf", status: stWork, assignee: exElectric, goodsOverride: []protoGood{g("22680-7S000", "1")}, logs: []demoLog{
				{exElectric, 3, "ДМРВ заменён, адаптация ХХ выполнена. Тест-драйв 20 минут — проверить ошибки."},
			}},
			{service: "throttle", status: stDone, assignee: exElectric},
		},
		media: []demoMedia{
			intakePhoto(32),
			{scene: mediaDash, name: "Ошибка на приборке", caption: "CHECK ENGINE, ПЛАВАЮТ ОБОРОТЫ", codes: "P0101", user: exIntake, hoursAgo: 32},
		},
	},
	{
		column: "В работе", clientIdx: 2, title: "Двигатель ВАЗ-21126 (снятый): капремонт с заменой колец и вкладышей", assignee: exMotor, dueOffset: offset(7),
		descHTML: "<p>Привезён снятым с Lada Priora. Со слов клиента: «масложор», стук, после обрыва ремня год назад меняли клапаны.</p>" +
			"<p>Дефектовка: кольца закоксованы, на 1-м цилиндре лёгкие задиры — хон, кольца STD, вкладыши, колпачки, полный комплект прокладок.</p>",
		equipment: []demoTaskEquipment{{key: "vaz21126", completeness: "Блок с ГБЦ, без навесного и КПП", condition: "Наружные следы масла, клапанная крышка треснута"}},
		subtasks: []demoSubtaskItem{
			{service: "overhaul", status: stWork, assignee: exMotor, priceOverride: "42000", logs: []demoLog{
				{exMotor, 30, "Разобрали: поршни целые, кольца закоксованы, на 1-м цилиндре лёгкие задиры. Блок на хон — размер STD сохраняем."},
				{exMotor, 6, "Блок с хона вернулся, коленвал в норме — вкладыши STD. Начали сборку."},
			}},
			{service: "valves", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{
			{scene: mediaBlock, name: "Агрегат на приёмке", caption: "ДВС 21126 НА СТЕНДЕ — ПРИЁМКА", user: exIntake, hoursAgo: 60},
			{scene: mediaBlock, name: "Дефектовка - цилиндр 1", caption: "ЦИЛ. 1 — ЗАДИРЫ ПОСЛЕ РАЗБОРКИ", user: exMotor, hoursAgo: 30},
		},
	},
	// --- Ждёт запчасти ---
	{
		column: "Ждёт запчасти", clientIdx: 3, title: "Toyota Camry М555АУ 177: эмульсия в масле, белый выхлоп — замена прокладки ГБЦ", assignee: exMotor, dueOffset: offset(5),
		descHTML:  "<p>После перегрева на трассе — белый густой выхлоп, антифриз уходит, на щупе эмульсия.</p><p>Прокладка ГБЦ 11115-36020 под заказ, поставка через 3 дня. Автомобиль на стоянке сервиса.</p>",
		equipment: []demoTaskEquipment{{key: "camry", completeness: "Ключ, СТС", condition: "Без повреждений кузова"}},
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exDiag},
			{service: "cooltest", status: stDone, assignee: exDiag, logs: []demoLog{
				{exDiag, 40, "Тест CO₂ положительный, в масле эмульсия. Компрессия 4-го цилиндра 9 бар против 13 в остальных."},
			}},
			{service: "headgasket", status: stNew, assignee: exMotor, dueOffset: offset(5), goodsOverride: []protoGood{g("11115-36020", "1"), g("RTV-GREY-80", "1")}, logs: []demoLog{
				{exAdmin, 18, "Прокладка 11115-36020 заказана у «АвтоДеталь-Опт», счёт оплачен, поставка в пятницу."},
			}},
			{service: "headmill", status: stNew, assignee: exNone},
			{service: "coolant", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("AF-G12P-RED", "7")}},
		},
		media: []demoMedia{
			intakePhoto(42),
			{scene: mediaDipstick, name: "Щуп - эмульсия", caption: "ЭМУЛЬСИЯ НА ЩУПЕ", user: exDiag, hoursAgo: 40},
		},
	},
	{
		column: "Ждёт запчасти", clientIdx: 5, title: "Hyundai Solaris Т110ХК 799 (такси): гремит цепь ГРМ при пуске, P0016", assignee: exMotor, dueOffset: offset(3),
		equipment: []demoTaskEquipment{{key: "taxi-solaris", completeness: "Ключ, путевой лист"}},
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exDiag, payout: "700", payoutPaid: true, logs: []demoLog{
				{exDiag, 40, "P0016 — рассогласование коленвала и распредвала. Фазы смещены на ~6°, цепь растянута."},
			}},
			{service: "noise", status: stDone, assignee: exDiag},
			{service: "chain", status: stNew, assignee: exMotor, logs: []demoLog{
				{exAdmin, 36, "Комплект цепи G4FG под заказ у поставщика, срок 3 дня. Таксопарк подтвердил ремонт письмом."},
			}},
		},
		media: []demoMedia{
			intakePhoto(44),
			{video: true, scene: mediaEngine, name: "Видео - звук цепи при пуске", caption: "ГРЕМИТ ЦЕПЬ ПРИ ПУСКЕ", markX: 0.22, markY: 0.40, shake: true, user: exDiag, hoursAgo: 42},
		},
	},
	// --- Готово ---
	{
		column: "Готово", clientIdx: 5, title: "Kia Rio Т245ХК 799 (такси): ТО-285 000, пропуски зажигания 3-го цилиндра", assignee: exElectric, dueOffset: offset(-2), paid: true,
		equipment: []demoTaskEquipment{{key: "taxi-rio", completeness: "Ключ, путевой лист"}},
		subtasks: []demoSubtaskItem{
			{service: "oil", status: stDone, assignee: exMotor, payout: "700", payoutPaid: true, goodsOverride: []protoGood{g("OIL-5W30-208", "3.6"), g("26300-35505", "1")}},
			{service: "plugs", status: stDone, assignee: exElectric, payout: "500", payoutPaid: true},
			{service: "coil", status: stDone, assignee: exElectric, payout: "400", logs: []demoLog{
				{exElectric, 54, "Катушка 3-го цилиндра пробита (переставили — пропуск ушёл за ней). Заменена, ошибок нет."},
			}},
			{service: "air", status: stDone, assignee: exMotor},
		},
		media: []demoMedia{intakePhoto(60)},
	},
	{
		column: "Готово", clientIdx: 6, title: "Lada Largus В946СА 750: замена ремня ГРМ с помпой, антифриз", assignee: exMotor, dueOffset: offset(-4), paid: true,
		equipment: []demoTaskEquipment{{key: "largus", completeness: "Ключ"}},
		subtasks: []demoSubtaskItem{
			{service: "timingpump", status: stDone, assignee: exMotor, payout: "3500", payoutPaid: true, logs: []demoLog{
				{exMotor, 100, "Ремень ГРМ и помпа заменены, метки выставлены, течи нет. Следующая замена — через 60 000 км."},
			}},
			{service: "coolant", status: stDone, assignee: exMotor, payout: "800", goodsOverride: []protoGood{g("AF-G11-GRN", "6")}},
		},
		media: []demoMedia{intakePhoto(110)},
	},
	{
		column: "Готово", clientIdx: 8, title: "Mitsubishi Outlander Х350ЕМ 97: неровный ХХ — промывка форсунок, свечи", assignee: exElectric, dueOffset: offset(-1),
		equipment: []demoTaskEquipment{{key: "outlander", completeness: "Ключ"}},
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exElectric, logs: []demoLog{
				{exIntake, 6, "Клиент оповещён по SMS, оплата при выдаче. Машина на стоянке, место №3."},
			}},
			{service: "injectors", status: stDone, assignee: exElectric, payout: "1200", logs: []demoLog{
				{exElectric, 28, "Форсунки на стенде: производительность 2-й и 4-й на 12% ниже. После УЗ-промывки разброс 2%."},
			}},
			{service: "plugs", status: stDone, assignee: exElectric},
		},
		media: []demoMedia{intakePhoto(50)},
	},
}
