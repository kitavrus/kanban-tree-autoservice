package seeddemo

import "github.com/kitavrus/kanban-tree/internal/features/board"

// Заказы автосервиса по колонкам доски (колонки сеет миграция 0005): «Приёмка» — авто только
// приняли либо записали на будущее; «Диагностика» — ищем причину; «Согласование» — смета у
// клиента; «Ждёт запчасти» — запчасти под заказ (остаток уходит в минус); «В ремонте»; «Готов к
// выдаче» — ждёт клиента и оплату; «Выдан» (финальная) — авто у клиента. Последние четыре
// списывают товар со склада.
//
// Заказ = ровно одна машина клиента (one-car-order); у таксопарка и доставки — по заказу на
// машину, часть машин клиентов без заказов. План работ (order-start-duration) подобран так,
// чтобы календарь загрузки при вместимости 8 машин был наглядным: сегодня занято 7 мест, в
// ближайшую неделю — 5–7, а через 5 дней — все 8 (см. TestValidateDemoData / seed-тест).

// demoSubtaskItem — услуга заказа: service — code из demoServices; assignee -1 — без
// исполнителя; dueOffset — срок услуги, дни от сегодня (внутри плана заказа);
// goodsOverride/priceOverride — вместо товаров и цены услуги по умолчанию. payout/payoutPaid —
// выплата мастеру (executor-payouts, D71).
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

// demoLog — запись журнала работ (subtask_logs). user — индекс demoExecutors, -1 — админ;
// hoursAgo — не раньше приёмки заказа и не позже выдачи.
type demoLog struct {
	user     int
	hoursAgo int
	text     string
}

// Сцены вложений (media.go).
const (
	mediaCar       = "car"       // авто спереди с госномером
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
//
// План работ (order-start-duration): startOffset — дни от сегодня до начала работ
// (отрицательные — машина уже в сервисе, положительные — запись на будущее), days — дней на
// работу (плановое окончание = начало + days − 1). acceptedH — сколько часов назад заказ
// заведён (created_at): у записи на будущее — когда записали. finishedOffset — день выдачи
// (только финальная колонка; ≤ −1 и не раньше начала): finished_at — в 17:30 по Алматы.
type demoTaskItem struct {
	column         string
	clientIdx      int
	title          string
	assignee       int
	startOffset    int
	days           int
	acceptedH      int
	finishedOffset int
	paid           bool
	descHTML       string
	car            string // ключ автомобиля заказа (demoEquipment.key) — ровно один на заказ
	subtasks       []demoSubtaskItem
	media          []demoMedia
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
	// --- Приёмка: записи на будущее (машины ещё нет — фото приёмки нет) и принятая сегодня ---
	{
		column: "Приёмка", clientIdx: 9, title: "Mazda CX-5 Е019ТТ 178: ТО-120 000 (масло, фильтр, свечи, дроссель)", assignee: exMotor,
		startOffset: 2, days: 1, acceptedH: 6,
		car: "cx5",
		subtasks: []demoSubtaskItem{
			{service: "oil", status: stNew, assignee: exMotor, dueOffset: offset(2), goodsOverride: []protoGood{g("OIL-5W30-208", "4.2"), g("PE01-14-302A", "1")}, logs: []demoLog{
				{exIntake, 6, "Клиент записан на ТО послезавтра к 10:00, будет ждать в зоне отдыха. Масло 5W-30 согласовано."},
			}},
			{service: "plugs", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("SK16HR11", "4")}},
			{service: "throttle", status: stNew, assignee: exMotor},
		},
	},
	{
		column: "Приёмка", clientIdx: 8, title: "Toyota Land Cruiser Prado С808ВО 77: ТО-165 000, топливный фильтр, рывки на разгоне", assignee: exMotor,
		startOffset: 1, days: 2, acceptedH: 26,
		car: "prado",
		subtasks: []demoSubtaskItem{
			{service: "oil", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("OIL-5W30-208", "7.5"), g("04152-YZZA6", "1")}},
			{service: "fuel", status: stNew, assignee: exMotor},
			{service: "diag", status: stNew, assignee: exElectric, logs: []demoLog{
				{exIntake, 26, "Записан на завтра 9:00. Клиент: на разгоне 1500–2000 об/мин лёгкие рывки, началось после заправки на трассе. Взять пробу топлива."},
			}},
		},
	},
	{
		column: "Приёмка", clientIdx: 5, title: "Renault Logan Т512ХК 799 (такси): ТО-345 000, регулировка клапанов", assignee: exMotor,
		startOffset: 5, days: 1, acceptedH: 20,
		car: "taxi-logan",
		subtasks: []demoSubtaskItem{
			{service: "oil", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("OIL-5W40-208", "4.8"), g("8200768913", "1")}, logs: []demoLog{
				{exAdmin, 20, "Таксопарк записал борт №31 на ТО и регулировку клапанов: пробег 345 000, на холодную стучат клапаны."},
			}},
			{service: "valves", status: stNew, assignee: exMotor},
		},
	},
	{
		column: "Приёмка", clientIdx: 6, title: "ГАЗель Next В734СА 750: тяжело заводится на холодную, белый дым", assignee: exNone,
		startOffset: 0, days: 2, acceptedH: 4,
		descHTML: "<p>Пригнали на буксире. Заводится с 5–6 попытки, первые минуты белый дым и вибрация, тяга слабая.</p><p>Подозрение: свечи накала / форсунки / компрессия.</p>",
		car:      "gazelle",
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stNew, assignee: exNone, dueOffset: offset(0), logs: []demoLog{
				{exIntake, 3, "Машина нужна клиенту к пятнице — развозка по объектам. Разрешили диагностику до 5 000 ₽ без согласования."},
			}},
			{service: "compr", status: stNew, assignee: exNone},
		},
		media: []demoMedia{
			intakePhoto(3),
			{scene: mediaEngine, name: "Подкапотное - копоть на впуске", caption: "КОПОТЬ НА ВПУСКЕ, ПОДТЁК ДИЗТОПЛИВА", markX: 0.30, markY: 0.55, user: exIntake, hoursAgo: 3},
		},
	},
	// --- Диагностика ---
	{
		column: "Диагностика", clientIdx: 1, title: "Hyundai Solaris О718МР 199: троит на холодную, Check Engine P0300", assignee: exDiag,
		startOffset: -1, days: 2, acceptedH: 26,
		descHTML: "<p>Со слов клиента: первые 3–5 минут после холодного пуска двигатель троит, на прогретом — ровно. Горит Check Engine.</p>" +
			"<ul><li>Пробег 187 000 км, свечи меняли ~40 000 км назад</li><li>Ошибки (сканер на заправке): P0300, P0302</li><li>Проверить свечи, катушки, компрессию 2-го цилиндра</li></ul>",
		car: "solaris",
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stWork, assignee: exDiag, dueOffset: offset(0), logs: []demoLog{
				{exIntake, 25, "Пробег 187 000, на приёмке горит CE, клиент жалуется на троение на холодную. Фото приборки и видео работы на ХХ приложены."},
				{exDiag, 4, "Пропуски во 2-м цилиндре на холодную. Свеча 2-го в нагаре, катушки переставили местами — ждём повтора после остывания."},
			}},
			{service: "compr", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{
			intakePhoto(25),
			{scene: mediaDash, name: "Приборная панель на приёмке", caption: "ГОРИТ CHECK ENGINE НА ПРИЁМКЕ", codes: "P0300  P0302", user: exIntake, hoursAgo: 25},
			{video: true, scene: mediaEngine, name: "Видео - троение на ХХ", caption: "ТРОИТ НА ХОЛОДНУЮ", markX: 0.42, markY: 0.45, shake: true, user: exIntake, hoursAgo: 25},
		},
	},
	// --- Согласование ---
	{
		column: "Согласование", clientIdx: 2, title: "Volkswagen Polo К902ТН 750: стук в двигателе, расход масла 1 л на 1000 км", assignee: exMotor,
		startOffset: -3, days: 12, acceptedH: 74,
		descHTML: "<p>Стук на прогретом двигателе под нагрузкой, сизый дым при перегазовке, расход масла ~1 л/1000 км.</p>" +
			"<p><strong>Дефектовка:</strong> компрессия 1–2–3–4: 12,5 / 12,0 / 8,5 / 12,0 бар; эндоскопия — задиры в 3-м цилиндре. Рекомендован капремонт, машина на стоянке сервиса.</p>",
		car: "polo",
		subtasks: []demoSubtaskItem{
			{service: "noise", status: stDone, assignee: exDiag, payout: "600", logs: []demoLog{
				{exDiag, 70, "Стук глухой, усиливается при прогазовке на 2500–3000 об/мин. Похоже на поршневую группу 3-го цилиндра."},
			}},
			{service: "compr", status: stDone, assignee: exMotor, logs: []demoLog{
				{exMotor, 68, "Компрессия 1-2-3-4: 12,5 / 12,0 / 8,5 / 12,0 бар. С маслом в 3-м поднялась до 10,5 — кольца."},
			}},
			{service: "endo", status: stDone, assignee: exDiag, logs: []demoLog{
				{exDiag, 67, "Эндоскоп: в 3-м цилиндре вертикальные задиры на стенке, хон стёрт. Фото приложено."},
			}},
			// запчастей VAG под капремонт на складе нет — заказываются после согласования
			{service: "overhaul", status: stNew, assignee: exMotor, priceOverride: "95000", goodsOverride: []protoGood{}, logs: []demoLog{
				{exAdmin, 48, "Смета на капремонт 95 000 ₽ + запчасти по факту отправлена клиенту в WhatsApp, ждём решения."},
			}},
			{service: "engineRR", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{
			intakePhoto(72),
			{scene: mediaEndoscope, name: "Эндоскопия - цилиндр 3", caption: "ЦИЛ. 3 — ВЕРТИКАЛЬНЫЕ ЗАДИРЫ", user: exDiag, hoursAgo: 67},
		},
	},
	{
		column: "Согласование", clientIdx: 7, title: "Fiat Ducato У603МН 190: течь масла, замена ремня ГРМ по регламенту", assignee: exMotor,
		startOffset: 3, days: 3, acceptedH: 30,
		descHTML: "<p>Пробег 402 600 км, ремень ГРМ меняли 120 000 км назад — пора по регламенту. Течь масла из-под шкива коленвала, пятно на стоянке.</p>" +
			"<p>Осмотр сделали вчера, машину клиент забрал на линию — ремонт записан на ближайшее окно.</p>",
		car: "ducato",
		subtasks: []demoSubtaskItem{
			{service: "noise", status: stDone, assignee: exMotor, payout: "500", payoutPaid: true, logs: []demoLog{
				{exMotor, 26, "Течь по переднему сальнику коленвала, ремень ГРМ с трещинами на тыльной стороне — менять обязательно."},
			}},
			{service: "timing", status: stNew, assignee: exMotor, dueOffset: offset(4), priceOverride: "9500", goodsOverride: []protoGood{g("KTB553", "1")}},
			{service: "seals", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("12019350B", "1")}, logs: []demoLog{
				{exAdmin, 20, "Предварительно записали на ремонт через 3 дня. Клиент согласовывает ГРМ + сальник, по деньгам думает до завтра."},
			}},
		},
		media: []demoMedia{
			intakePhoto(28),
			{scene: mediaEngine, name: "Течь по сальнику коленвала", caption: "ТЕЧЬ ПО ПЕРЕДНЕМУ САЛЬНИКУ КОЛЕНВАЛА", markX: 0.25, markY: 0.75, user: exMotor, hoursAgo: 26},
		},
	},
	// --- В ремонте ---
	{
		column: "В ремонте", clientIdx: 5, title: "Skoda Octavia Т389ХК 799 (такси): плавают обороты ХХ + ремень ГРМ по пробегу", assignee: exMotor,
		startOffset: -2, days: 3, acceptedH: 50,
		car: "taxi-octavia",
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exDiag, payout: "700", payoutPaid: true, logs: []demoLog{
				{exDiag, 46, "Ошибки P0507 (обороты ХХ выше нормы), P2187 (бедная смесь на ХХ). Похоже на подсос воздуха."},
			}},
			{service: "smoke", status: stDone, assignee: exDiag, payout: "700", logs: []demoLog{
				{exDiag, 44, "Дымогенератор: трещина патрубка вентиляции картерных газов у клапанной крышки. Патрубок таксопарк купил сам."},
			}},
			{service: "timing", status: stWork, assignee: exMotor, dueOffset: offset(0), goodsOverride: []protoGood{g("CT1168K1", "1")}, logs: []demoLog{
				{exMotor, 4, "Сняли старый ремень — износ по кромке, ролик с люфтом. Ставим новый ремень с роликом, метки совпали."},
			}},
		},
		media: []demoMedia{
			intakePhoto(49),
			{scene: mediaEngine, name: "Дымогенератор - подсос", caption: "ТРЕЩИНА ПАТРУБКА ВКГ (ДЫМ)", markX: 0.55, markY: 0.30, steam: true, user: exDiag, hoursAgo: 44},
		},
	},
	{
		column: "В ремонте", clientIdx: 0, title: "Lada Vesta Е245КХ 797: перегрев, уходит антифриз — пробита прокладка ГБЦ", assignee: exMotor,
		startOffset: -5, days: 4, acceptedH: 122,
		descHTML: "<p>Перегрев в пробке, стрелка в красной зоне, пар из-под капота. Антифриз уходит ~0,5 л в день, внешних течей нет.</p>" +
			"<ul><li>Тест CO₂ — положительный, пузыри в расширительном бачке</li><li>ГБЦ — в мехцех на опрессовку и проверку плоскости</li></ul>",
		car: "vesta",
		subtasks: []demoSubtaskItem{
			{service: "cooltest", status: stDone, assignee: exDiag, payout: "900", payoutPaid: true, logs: []demoLog{
				{exDiag, 118, "Опрессовка 1,4 бар — давление падает за 10 минут, внешних течей нет. Тест CO₂ в бачке пожелтел — выхлоп в системе охлаждения."},
			}},
			{service: "headgasket", status: stWork, assignee: exMotor, dueOffset: offset(-2), logs: []demoLog{
				{exMotor, 96, "ГБЦ снята. Прокладка прогорела между 2-м и 3-м цилиндром, фото приложено. Головка ушла в мехцех."},
			}},
			{service: "headmill", status: stWork, assignee: exNone, logs: []demoLog{
				{exAdmin, 94, "ГБЦ отдана в мехцех «ГБЦ-Сервис», обещали готовность через два дня."},
				{exAdmin, 5, "Мехцех нашёл трещину между сёдлами клапанов — ищут другую головку. Клиенту сообщили, срок ремонта сдвигается."},
			}},
			{service: "coolant", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{
			intakePhoto(121),
			{video: true, scene: mediaTank, name: "Видео - пузыри в расширительном бачке", caption: "ПУЗЫРИ В БАЧКЕ", user: exDiag, hoursAgo: 118},
			{scene: mediaGasket, name: "Снятая прокладка ГБЦ", caption: "ПРОГАР ПРОКЛАДКИ МЕЖДУ 2 И 3 ЦИЛИНДРОМ", user: exMotor, hoursAgo: 96},
		},
	},
	{
		column: "В ремонте", clientIdx: 2, title: "Lada Priora Р126ОР 750: капремонт двигателя ВАЗ-21126 (привезён снятым)", assignee: exMotor,
		startOffset: -6, days: 12, acceptedH: 146,
		descHTML: "<p>Двигатель привезён снятым с машины, без навесного и КПП. Со слов клиента: «масложор», стук, после обрыва ремня год назад меняли клапаны.</p>" +
			"<p>Дефектовка: кольца закоксованы, на 1-м цилиндре лёгкие задиры — хон, кольца STD, вкладыши, колпачки, полный набор прокладок.</p>",
		car: "priora",
		subtasks: []demoSubtaskItem{
			{service: "overhaul", status: stWork, assignee: exMotor, dueOffset: offset(5), priceOverride: "42000", logs: []demoLog{
				{exMotor, 120, "Разобрали: поршни целые, кольца закоксованы, на 1-м цилиндре лёгкие задиры. Блок на хон — размер STD сохраняем."},
				{exMotor, 20, "Блок с хона вернулся, коленвал в норме — вкладыши STD. Начали сборку."},
			}},
			{service: "valves", status: stNew, assignee: exMotor},
		},
		media: []demoMedia{
			{scene: mediaBlock, name: "Двигатель на приёмке", caption: "ДВС 21126 НА СТЕНДЕ — ПРИЁМКА", user: exIntake, hoursAgo: 145},
			{scene: mediaBlock, name: "Дефектовка - цилиндр 1", caption: "ЦИЛ. 1 — ЗАДИРЫ ПОСЛЕ РАЗБОРКИ", user: exMotor, hoursAgo: 120},
		},
	},
	// --- Ждёт запчасти: машины уехали до поставки, ремонт записан к её дню ---
	{
		column: "Ждёт запчасти", clientIdx: 3, title: "Toyota Camry М555АУ 177: эмульсия в масле, белый выхлоп — замена прокладки ГБЦ", assignee: exMotor,
		startOffset: 3, days: 4, acceptedH: 44,
		descHTML: "<p>После перегрева на трассе — белый густой выхлоп, антифриз уходит, на щупе эмульсия.</p>" +
			"<p>Прокладка ГБЦ 11115-36020 под заказ, поставка через 3 дня. Клиент забрал машину до поставки (ездит аккуратно, доливает антифриз), ремонт записан на день поставки.</p>",
		car: "camry",
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exDiag},
			{service: "cooltest", status: stDone, assignee: exDiag, logs: []demoLog{
				{exDiag, 40, "Тест CO₂ положительный, в масле эмульсия. Компрессия 4-го цилиндра 9 бар против 13 в остальных."},
			}},
			{service: "headgasket", status: stNew, assignee: exMotor, dueOffset: offset(4), goodsOverride: []protoGood{g("11115-36020", "1"), g("RTV-GREY-80", "1")}, logs: []demoLog{
				{exAdmin, 18, "Прокладка 11115-36020 заказана у «АвтоДеталь-Опт», счёт оплачен, поставка через 3 дня."},
			}},
			{service: "headmill", status: stNew, assignee: exNone},
			{service: "coolant", status: stNew, assignee: exMotor, goodsOverride: []protoGood{g("AF-G12P-RED", "7")}},
		},
		media: []demoMedia{
			intakePhoto(43),
			{scene: mediaDipstick, name: "Щуп - эмульсия", caption: "ЭМУЛЬСИЯ НА ЩУПЕ", user: exDiag, hoursAgo: 40},
		},
	},
	{
		column: "Ждёт запчасти", clientIdx: 5, title: "Hyundai Solaris Т110ХК 799 (такси): гремит цепь ГРМ при пуске, P0016", assignee: exMotor,
		startOffset: 4, days: 3, acceptedH: 46,
		car: "taxi-solaris",
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exDiag, payout: "700", payoutPaid: true, logs: []demoLog{
				{exDiag, 40, "P0016 — рассогласование коленвала и распредвала. Фазы смещены на ~6°, цепь растянута."},
			}},
			{service: "noise", status: stDone, assignee: exDiag},
			{service: "chain", status: stNew, assignee: exMotor, logs: []demoLog{
				{exAdmin, 36, "Цепь G4FG с натяжителем под заказ у поставщика, срок 3 дня. Таксопарк забрал машину на линию до поставки, ремонт записан."},
			}},
		},
		media: []demoMedia{
			intakePhoto(45),
			{video: true, scene: mediaEngine, name: "Видео - звук цепи при пуске", caption: "ГРЕМИТ ЦЕПЬ ПРИ ПУСКЕ", markX: 0.22, markY: 0.40, shake: true, user: exDiag, hoursAgo: 42},
		},
	},
	// --- Готов к выдаче: план прошёл, клиент не забирает (просрочено) ---
	{
		column: "Готов к выдаче", clientIdx: 4, title: "Nissan Qashqai Н481ОР 50: плавают обороты, ошибка P0101 (ДМРВ)", assignee: exElectric,
		startOffset: -3, days: 2, acceptedH: 74,
		car: "qashqai",
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exElectric, payout: "800", payoutPaid: true, logs: []demoLog{
				{exElectric, 70, "P0101 — ДМРВ вне диапазона. На ХХ 2,1 г/с при норме 2,6–3,0; на прогазовке не растёт. Разъём чистый."},
			}},
			{service: "maf", status: stDone, assignee: exElectric, payout: "600", goodsOverride: []protoGood{g("22680-7S000", "1")}, logs: []demoLog{
				{exElectric, 50, "ДМРВ заменён, адаптация ХХ выполнена."},
				{exElectric, 48, "Тест-драйв 20 минут: обороты ровные, ошибок нет. Клиенту позвонили — заберёт вечером."},
				{exAdmin, 6, "Клиент второй день не забирает машину — в командировке, обещал в пятницу. Машина на стоянке, место №3."},
			}},
			{service: "throttle", status: stDone, assignee: exElectric},
		},
		media: []demoMedia{
			intakePhoto(73),
			{scene: mediaDash, name: "Ошибка на приборке", caption: "CHECK ENGINE, ПЛАВАЮТ ОБОРОТЫ", codes: "P0101", user: exIntake, hoursAgo: 72},
		},
	},
	// --- Выдан ---
	{
		column: "Выдан", clientIdx: 8, title: "Mitsubishi Outlander Х350ЕМ 97: неровный ХХ — промывка форсунок, свечи", assignee: exElectric,
		startOffset: -3, days: 2, acceptedH: 74, finishedOffset: -1, paid: true,
		car: "outlander",
		subtasks: []demoSubtaskItem{
			{service: "diag", status: stDone, assignee: exElectric, logs: []demoLog{
				{exIntake, 52, "Клиент оповещён по SMS, оплата при выдаче. Заберёт завтра после работы."},
			}},
			{service: "injectors", status: stDone, assignee: exElectric, payout: "1200", logs: []demoLog{
				{exElectric, 60, "Форсунки на стенде: производительность 2-й и 4-й на 12% ниже. После УЗ-промывки разброс 2%."},
			}},
			{service: "plugs", status: stDone, assignee: exElectric},
		},
		media: []demoMedia{intakePhoto(73)},
	},
	{
		column: "Выдан", clientIdx: 5, title: "Kia Rio Т245ХК 799 (такси): ТО-285 000, пропуски зажигания 3-го цилиндра", assignee: exElectric,
		startOffset: -7, days: 2, acceptedH: 170, finishedOffset: -6, paid: true,
		car: "taxi-rio",
		subtasks: []demoSubtaskItem{
			{service: "oil", status: stDone, assignee: exMotor, payout: "700", payoutPaid: true, goodsOverride: []protoGood{g("OIL-5W30-208", "3.6"), g("26300-35505", "1")}},
			{service: "plugs", status: stDone, assignee: exElectric, payout: "500", payoutPaid: true},
			{service: "coil", status: stDone, assignee: exElectric, payout: "400", logs: []demoLog{
				{exElectric, 155, "Катушка 3-го цилиндра пробита (переставили — пропуск ушёл за ней). Заменена, ошибок нет."},
			}},
			{service: "air", status: stDone, assignee: exMotor},
		},
		media: []demoMedia{intakePhoto(169)},
	},
	{
		column: "Выдан", clientIdx: 6, title: "Lada Largus В946СА 750: замена ремня ГРМ с помпой, антифриз", assignee: exMotor,
		startOffset: -12, days: 3, acceptedH: 290, finishedOffset: -10, paid: true,
		car: "largus",
		subtasks: []demoSubtaskItem{
			{service: "timingpump", status: stDone, assignee: exMotor, payout: "3500", payoutPaid: true, logs: []demoLog{
				{exMotor, 250, "Ремень ГРМ и помпа заменены, метки выставлены, течи нет. Следующая замена — через 60 000 км."},
			}},
			{service: "coolant", status: stDone, assignee: exMotor, payout: "800", goodsOverride: []protoGood{g("AF-G11-GRN", "6")}},
		},
		media: []demoMedia{intakePhoto(289)},
	},
}
