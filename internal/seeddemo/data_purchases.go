package seeddemo

// Закупки у поставщиков (purchase-orders): заказы во всех колонках канбана «Закупки» —
// новые черновики, отправленные без ответа, с расхождениями (поставщик соберёт меньше),
// изменённые после отправки, собранные, забранные и отменённый. Поставщик «АвтоДеталь-Опт»
// получает вход в кабинет поставщика (demoSupplierLogin / demoPassword).

// demoSupplierLogin — учётка кабинета поставщика «АвтоДеталь-Опт».
const demoSupplierLogin = "avtodetal"

// demoPOLine — строка заказа: артикул, сколько просим, ответ поставщика (answer=false — ещё
// не ответил; qty "0" — нет в наличии).
type demoPOLine struct {
	sku, qty, reason string
	answer           bool
	supQty, supPrice string
	supNote          string
}

// demoPurchaseOrder — заказ поставщику. supplier "" — «Базар (сами)». Время — часы назад
// от момента прогона: создан, отправлен (0 — не отправлялся), собран, забран, отменён.
// changedAfterSend — заказчик правил первую строку и примечание после отправки: ответ
// поставщика по строке сброшен, ждём новый (как при правке в sent, DS-034/DS-048).
type demoPurchaseOrder struct {
	supplier, status, note  string
	author                  int // индекс demoExecutors; exAdmin — администратор
	createdH, sentH, readyH int
	pickedH, cancelledH     int
	cancelReason            string
	changedAfterSend        bool
	lines                   []demoPOLine
}

const (
	supAvtodetal = "АвтоДеталь-Опт"
	supZapchast  = "ЗапчастьСервис"
	supLiga      = "АвтоЛига"
	supMotor     = "МоторЗапчасть" // архивный
)

var demoPurchaseOrders = []demoPurchaseOrder{
	// Новые — черновики.
	{supplier: supAvtodetal, status: "draft", author: exAdmin, createdH: 1,
		note: "Фильтры на неделю — к пятнице",
		lines: []demoPOLine{
			{sku: "26300-35505", qty: "10", reason: "below_min"},
			{sku: "W 712/95", qty: "6", reason: "below_min"},
			{sku: "21080-1012005", qty: "12", reason: "manual"},
		}},
	{supplier: "", status: "draft", author: exIntake, createdH: 3,
		note: "Купить на рынке по дороге",
		lines: []demoPOLine{
			{sku: "RTV-GREY-80", qty: "3", reason: "manual"},
			{sku: "TB-CLEAN-400", qty: "4", reason: "manual"},
		}},
	// Выполняются — отправлен, поставщик ещё не ответил.
	{supplier: supZapchast, status: "sent", author: exAdmin, createdH: 6, sentH: 5,
		note: "Под заказы ГРМ — нужно завтра до обеда",
		lines: []demoPOLine{
			{sku: "CT1168K1", qty: "2", reason: "for_orders"},
			{sku: "KTB553", qty: "1", reason: "for_orders"},
			{sku: "24321-2B000-K", qty: "1", reason: "for_orders"},
		}},
	// С расхождением — поставщик соберёт меньше, по одной позиции нет в наличии.
	{supplier: supAvtodetal, status: "sent", author: exMotor, createdH: 30, sentH: 28,
		note: "Свечи и катушки под два заказа",
		lines: []demoPOLine{
			{sku: "BKR6EIX-11", qty: "8", reason: "for_orders", answer: true, supQty: "4", supPrice: "1150", supNote: "Остальные 4 — в четверг"},
			{sku: "27301-2B010", qty: "2", reason: "for_orders", answer: true, supQty: "2", supPrice: "4200"},
			{sku: "SK16HR11", qty: "4", reason: "below_min", answer: true, supQty: "0", supPrice: "0", supNote: "Нет в наличии, аналог — NGK"},
		}},
	// Изменён — после ответа поставщика заказчик увеличил количество и поправил примечание.
	{supplier: supLiga, status: "sent", author: exAdmin, createdH: 50, sentH: 48, changedAfterSend: true,
		note: "Антифриз и термостат — добавили помпу, заберём сами",
		lines: []demoPOLine{
			{sku: "AF-G12P-RED", qty: "40", reason: "below_min"}, // было 20 — ответ сброшен правкой
			{sku: "25100-2B000", qty: "1", reason: "for_orders", answer: true, supQty: "1", supPrice: "6900"},
		}},
	// Собран — всё как просили.
	{supplier: supZapchast, status: "ready", author: exAdmin, createdH: 72, sentH: 70, readyH: 20,
		note: "Масло и фильтры на ТО",
		lines: []demoPOLine{
			{sku: "OIL-5W30-208", qty: "60", reason: "below_min", answer: true, supQty: "60", supPrice: "650"},
			{sku: "04152-YZZA6", qty: "4", reason: "below_min", answer: true, supQty: "4", supPrice: "820"},
		}},
	// Собран с расхождением — привезут меньше прокладок.
	{supplier: supAvtodetal, status: "ready", author: exMotor, createdH: 96, sentH: 94, readyH: 30,
		note: "Прокладки ГБЦ под капремонт",
		lines: []demoPOLine{
			{sku: "21126-1003020", qty: "2", reason: "for_orders", answer: true, supQty: "1", supPrice: "3900", supNote: "Одна в наличии"},
			{sku: "22441-2B002", qty: "2", reason: "for_orders", answer: true, supQty: "2", supPrice: "1450"},
		}},
	// Завершены.
	{supplier: supAvtodetal, status: "picked_up", author: exAdmin, createdH: 170, sentH: 168, readyH: 150, pickedH: 140,
		note: "Плановая закупка расходников",
		lines: []demoPOLine{
			{sku: "INJ-CLEAN-1L", qty: "6", reason: "below_min", answer: true, supQty: "6", supPrice: "780"},
			{sku: "ENG-FLUSH-300", qty: "10", reason: "below_min", answer: true, supQty: "10", supPrice: "340"},
		}},
	{supplier: "", status: "picked_up", author: exIntake, createdH: 220, pickedH: 216,
		note: "Купили сами на Южном рынке",
		lines: []demoPOLine{
			{sku: "6PK1070", qty: "2", reason: "manual"},
		}},
	{supplier: supMotor, status: "cancelled", author: exAdmin, createdH: 300, sentH: 298, cancelledH: 260,
		cancelReason: "Поставщик сорвал сроки — заказали в «АвтоДеталь-Опт»",
		lines: []demoPOLine{
			{sku: "0280218116", qty: "1", reason: "for_orders"},
		}},
}
