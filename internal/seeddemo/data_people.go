package seeddemo

// Демо-данные автосервиса (ремонт двигателей): мастера, клиенты, типы авто и автомобили.
// Клиенты, телефоны, VIN и госномера — вымышленные.

// demoExecutor — мастера-исполнители (admin не создаётся: существующий не трогается — его
// пояс остаётся умолчанием Asia/Almaty). timezone пуст — пояс по умолчанию; один мастер — в
// Москве (user-timezone, DS-033: демо показывает оба пояса).
type demoExecutor struct{ login, name, phone, timezone string }

var demoExecutors = []demoExecutor{
	{login: "mironov", name: "Миронов Сергей", phone: "+7 916 400-00-01"},                        // 0 — моторист
	{login: "belov", name: "Белов Андрей", phone: "+7 916 400-00-02", timezone: "Europe/Moscow"}, // 1 — диагност
	{login: "zaytsev", name: "Зайцев Дмитрий", phone: "+7 916 400-00-03"},                        // 2 — автоэлектрик
	{login: "kuznetsova", name: "Кузнецова Ольга", phone: "+7 916 400-00-04"},                    // 3 — мастер-приёмщик
}

// Роли мастеров (индексы demoExecutors) — для читаемости заказов ниже.
const (
	exMotor    = 0  // моторист
	exDiag     = 1  // диагност
	exElectric = 2  // автоэлектрик
	exIntake   = 3  // мастер-приёмщик
	exAdmin    = -1 // администратор (seed-админ)
	exNone     = -1 // без исполнителя
)

// demoPhone — телефон клиента.
type demoPhone struct{ phone, note string }

// demoClient — частные владельцы и компании с автопарком (такси, доставка, перевозки).
type demoClient struct {
	company, address, contact, position string
	phones                              []demoPhone
}

var demoClients = []demoClient{
	{company: "Смирнов Алексей Игоревич", address: "г. Москва, ул. Профсоюзная, 88", contact: "Смирнов Алексей", position: "Владелец авто",
		phones: []demoPhone{{"+7 916 123-45-67", "мобильный"}}}, // 0
	{company: "Петрова Наталья Сергеевна", address: "г. Москва, ул. Академика Янгеля, 6", contact: "Петрова Наталья", position: "Владелец авто",
		phones: []demoPhone{{"+7 903 555-12-34", "WhatsApp"}}}, // 1
	{company: "Ковалёв Денис Андреевич", address: "г. Подольск, ул. Кирова, 15", contact: "Ковалёв Денис", position: "Владелец авто",
		phones: []demoPhone{{"+7 926 700-11-22", "мобильный"}, {"+7 496 755-10-10", "рабочий"}}}, // 2
	{company: "Ибрагимов Руслан Маратович", address: "г. Москва, Варшавское ш., 140", contact: "Ибрагимов Руслан", position: "Владелец авто",
		phones: []demoPhone{{"+7 985 210-33-44", "Telegram"}}}, // 3
	{company: "Орлова Екатерина Викторовна", address: "г. Видное, Битцевский пр., 3", contact: "Орлова Екатерина", position: "Владелец авто",
		phones: []demoPhone{{"+7 915 044-55-66", "звонить после 18:00"}}}, // 4
	{company: "ООО «ТаксиЛайн»", address: "г. Москва, ул. Подольских Курсантов, 22, автобаза", contact: "Гришин Павел Олегович", position: "Начальник автоколонны",
		phones: []demoPhone{{"+7 495 120-30-40", "диспетчерская"}, {"+7 916 800-90-90", "Гришин, мобильный"}}}, // 5
	{company: "ООО «СтройДоставка»", address: "г. Москва, Каширское ш., 61, склад 4", contact: "Лаптев Николай", position: "Механик автопарка",
		phones: []demoPhone{{"+7 495 330-44-55", "офис"}, {"+7 925 330-44-56", "механик"}}}, // 6
	{company: "ИП Захаров В. Н. (грузоперевозки)", address: "г. Домодедово, ул. Логистическая, 1", contact: "Захаров Виктор", position: "Владелец",
		phones: []demoPhone{{"+7 977 610-20-30", "WhatsApp"}}}, // 7
	{company: "Волков Михаил Петрович", address: "Московская обл., пос. Развилка, 12", contact: "Волков Михаил", position: "Владелец авто",
		phones: []demoPhone{{"+7 903 777-00-77", "мобильный"}, {"+7 903 777-00-78", "супруга"}}}, // 8
	{company: "Никитина Анна Олеговна", address: "г. Москва, ул. Россошанская, 4", contact: "Никитина Анна", position: "Владелец авто",
		phones: []demoPhone{{"+7 999 101-20-30", ""}}}, // 9
}

// demoEquipmentType — типы «оборудования» = типы автомобилей (keys → spec_keys).
type demoEquipmentType struct {
	name string
	keys []string
}

const (
	typeCar        = 0
	typeCrossover  = 1
	typeCommercial = 2
	typeEngine     = 3
)

var demoEquipmentTypes = []demoEquipmentType{
	{name: "Легковой автомобиль", keys: []string{"Год выпуска", "Двигатель", "Топливо", "КПП", "Пробег", "Цвет"}},                             // 0
	{name: "Кроссовер / внедорожник", keys: []string{"Год выпуска", "Двигатель", "Топливо", "КПП", "Привод", "Пробег", "Цвет"}},               // 1
	{name: "Коммерческий / микроавтобус", keys: []string{"Год выпуска", "Двигатель", "Топливо", "КПП", "Грузоподъёмность", "Пробег", "Цвет"}}, // 2
	{name: "Двигатель (снятый агрегат)", keys: []string{"Код двигателя", "Объём", "Номер блока", "Состояние"}},                                // 3
}

// specJSON — jsonb-элемент specs {"k":...,"v":...} (эталон clients.SpecDTO: json-теги k/v).
type specJSON struct {
	K string `json:"k"`
	V string `json:"v"`
}

type sp = specJSON

// demoEquipmentItem — автомобили клиентов. key — внутренний ключ для ссылок из заказов;
// serial — VIN (для снятого двигателя — номер блока); plate — госномер (equipment.
// plate_number, формат «А123ВС 777»; у снятого агрегата пуст).
type demoEquipmentItem struct {
	key                         string
	clientIdx, typeIdx          int
	brand, model, serial, plate string
	note                        string
	specs                       []sp
}

// specValue — значение характеристики по ключу.
func (e demoEquipmentItem) specValue(k string) string {
	for _, s := range e.specs {
		if s.K == k {
			return s.V
		}
	}
	return ""
}

var demoEquipment = []demoEquipmentItem{
	{key: "vesta", clientIdx: 0, typeIdx: typeCar, brand: "Lada", model: "Vesta 1.6", serial: "XTAGFL110KY234567", plate: "Е245КХ 797",
		specs: []sp{{"Год выпуска", "2019"}, {"Двигатель", "ВАЗ-21129, 1.6 л, 106 л.с."}, {"Топливо", "бензин АИ-95"}, {"КПП", "МКПП-5"}, {"Пробег", "142 300 км"}, {"Цвет", "серебристый"}}},
	{key: "solaris", clientIdx: 1, typeIdx: typeCar, brand: "Hyundai", model: "Solaris 1.6", serial: "Z94CU41CBFR412873", plate: "О718МР 199",
		specs: []sp{{"Год выпуска", "2015"}, {"Двигатель", "G4FC, 1.6 л, 123 л.с."}, {"Топливо", "бензин АИ-92"}, {"КПП", "АКПП-4"}, {"Пробег", "187 000 км"}, {"Цвет", "белый"}}},
	{key: "polo", clientIdx: 2, typeIdx: typeCar, brand: "Volkswagen", model: "Polo седан 1.6", serial: "XW8ZZZ61ZEG058214", plate: "К902ТН 750",
		specs: []sp{{"Год выпуска", "2014"}, {"Двигатель", "CFNA, 1.6 л, 105 л.с."}, {"Топливо", "бензин АИ-95"}, {"КПП", "МКПП-5"}, {"Пробег", "213 400 км"}, {"Цвет", "синий"}}},
	{key: "vaz21126", clientIdx: 2, typeIdx: typeEngine, brand: "ВАЗ", model: "Двигатель 21126 1.6 16V (от Priora)", serial: "7345612",
		note:  "Привезён снятым с Lada Priora — клиент восстанавливает машину для дочери",
		specs: []sp{{"Код двигателя", "21126"}, {"Объём", "1.6 л, 16 кл."}, {"Номер блока", "7345612"}, {"Состояние", "без навесного и КПП"}}},
	{key: "camry", clientIdx: 3, typeIdx: typeCar, brand: "Toyota", model: "Camry 2.5", serial: "XW7BF4FK10S123456", plate: "М555АУ 177",
		specs: []sp{{"Год выпуска", "2017"}, {"Двигатель", "2AR-FE, 2.5 л, 181 л.с."}, {"Топливо", "бензин АИ-95"}, {"КПП", "АКПП-6"}, {"Пробег", "158 900 км"}, {"Цвет", "чёрный"}}},
	{key: "sportage", clientIdx: 3, typeIdx: typeCrossover, brand: "Kia", model: "Sportage 2.0", serial: "XWEPH81ADJ0012345", plate: "Р307ЕК 777",
		specs: []sp{{"Год выпуска", "2018"}, {"Двигатель", "G4NA, 2.0 л, 150 л.с."}, {"Топливо", "бензин АИ-95"}, {"КПП", "АКПП-6"}, {"Привод", "4WD"}, {"Пробег", "96 200 км"}, {"Цвет", "серый"}}},
	{key: "qashqai", clientIdx: 4, typeIdx: typeCrossover, brand: "Nissan", model: "Qashqai 2.0", serial: "SJNFAAJ11U1234567", plate: "Н481ОР 50",
		specs: []sp{{"Год выпуска", "2016"}, {"Двигатель", "MR20DD, 2.0 л, 144 л.с."}, {"Топливо", "бензин АИ-95"}, {"КПП", "вариатор"}, {"Привод", "2WD"}, {"Пробег", "121 500 км"}, {"Цвет", "красный"}}},
	{key: "taxi-solaris", clientIdx: 5, typeIdx: typeCar, brand: "Hyundai", model: "Solaris 1.6", serial: "Z94K241CBLR198765", plate: "Т110ХК 799", note: "Такси, борт №14",
		specs: []sp{{"Год выпуска", "2020"}, {"Двигатель", "G4FG, 1.6 л, 123 л.с."}, {"Топливо", "бензин АИ-92"}, {"КПП", "АКПП-6"}, {"Пробег", "312 700 км"}, {"Цвет", "жёлтый"}}},
	{key: "taxi-rio", clientIdx: 5, typeIdx: typeCar, brand: "Kia", model: "Rio 1.6", serial: "Z94C251ABKR276431", plate: "Т245ХК 799", note: "Такси, борт №22",
		specs: []sp{{"Год выпуска", "2019"}, {"Двигатель", "G4FG, 1.6 л, 123 л.с."}, {"Топливо", "бензин АИ-92"}, {"КПП", "АКПП-6"}, {"Пробег", "284 100 км"}, {"Цвет", "жёлтый"}}},
	{key: "taxi-octavia", clientIdx: 5, typeIdx: typeCar, brand: "Skoda", model: "Octavia 1.4 TSI", serial: "XW8AN2NE7JH012345", plate: "Т389ХК 799", note: "Такси «комфорт», борт №7",
		specs: []sp{{"Год выпуска", "2018"}, {"Двигатель", "CZDA, 1.4 TSI, 150 л.с."}, {"Топливо", "бензин АИ-95"}, {"КПП", "DSG-7"}, {"Пробег", "265 800 км"}, {"Цвет", "белый"}}},
	{key: "taxi-logan", clientIdx: 5, typeIdx: typeCar, brand: "Renault", model: "Logan 1.6", serial: "X7L4SRAV458123456", plate: "Т512ХК 799", note: "Такси, борт №31, ГБО",
		specs: []sp{{"Год выпуска", "2017"}, {"Двигатель", "K7M, 1.6 л, 82 л.с."}, {"Топливо", "бензин / ГБО (пропан)"}, {"КПП", "МКПП-5"}, {"Пробег", "341 000 км"}, {"Цвет", "жёлтый"}}},
	{key: "gazelle", clientIdx: 6, typeIdx: typeCommercial, brand: "ГАЗ", model: "ГАЗель Next A21R22", serial: "X96A21R22K2765432", plate: "В734СА 750",
		specs: []sp{{"Год выпуска", "2019"}, {"Двигатель", "Cummins ISF 2.8, 149 л.с."}, {"Топливо", "дизель"}, {"КПП", "МКПП-5"}, {"Грузоподъёмность", "1,5 т"}, {"Пробег", "236 400 км"}, {"Цвет", "белый"}}},
	{key: "transit", clientIdx: 6, typeIdx: typeCommercial, brand: "Ford", model: "Transit 2.2 TDCi", serial: "Z6FXXXESGXGR12345", plate: "В118СА 750",
		specs: []sp{{"Год выпуска", "2016"}, {"Двигатель", "2.2 TDCi, 125 л.с."}, {"Топливо", "дизель"}, {"КПП", "МКПП-6"}, {"Грузоподъёмность", "1,2 т"}, {"Пробег", "298 000 км"}, {"Цвет", "белый"}}},
	{key: "largus", clientIdx: 6, typeIdx: typeCommercial, brand: "Lada", model: "Largus фургон 1.6", serial: "XTARS0Y5LL0712345", plate: "В946СА 750",
		specs: []sp{{"Год выпуска", "2020"}, {"Двигатель", "K4M, 1.6 л 16 кл., 102 л.с."}, {"Топливо", "бензин АИ-92"}, {"КПП", "МКПП-5"}, {"Грузоподъёмность", "0,8 т"}, {"Пробег", "174 300 км"}, {"Цвет", "белый"}}},
	{key: "ducato", clientIdx: 7, typeIdx: typeCommercial, brand: "Fiat", model: "Ducato 2.3 Multijet", serial: "ZFA25000002765431", plate: "У603МН 190",
		specs: []sp{{"Год выпуска", "2015"}, {"Двигатель", "F1AE (2.3 Multijet), 130 л.с."}, {"Топливо", "дизель"}, {"КПП", "МКПП-6"}, {"Грузоподъёмность", "1,5 т"}, {"Пробег", "402 600 км"}, {"Цвет", "серебристый"}}},
	{key: "prado", clientIdx: 8, typeIdx: typeCrossover, brand: "Toyota", model: "Land Cruiser Prado 150 2.8", serial: "JTEBR3FJ70K123456", plate: "С808ВО 77",
		specs: []sp{{"Год выпуска", "2018"}, {"Двигатель", "1GD-FTV, 2.8 D-4D, 177 л.с."}, {"Топливо", "дизель"}, {"КПП", "АКПП-6"}, {"Привод", "4WD"}, {"Пробег", "167 000 км"}, {"Цвет", "чёрный"}}},
	{key: "outlander", clientIdx: 8, typeIdx: typeCrossover, brand: "Mitsubishi", model: "Outlander 2.4", serial: "Z8TXTGF3WEM012345", plate: "Х350ЕМ 97",
		specs: []sp{{"Год выпуска", "2014"}, {"Двигатель", "4B12, 2.4 л, 167 л.с."}, {"Топливо", "бензин АИ-95"}, {"КПП", "вариатор"}, {"Привод", "4WD"}, {"Пробег", "205 300 км"}, {"Цвет", "бежевый"}}},
	{key: "cx5", clientIdx: 9, typeIdx: typeCrossover, brand: "Mazda", model: "CX-5 2.0", serial: "RUMKE1978HV012345", plate: "Е019ТТ 178",
		specs: []sp{{"Год выпуска", "2017"}, {"Двигатель", "PE-VPS (Skyactiv-G 2.0), 150 л.с."}, {"Топливо", "бензин АИ-95"}, {"КПП", "АКПП-6"}, {"Привод", "2WD"}, {"Пробег", "119 800 км"}, {"Цвет", "серый"}}},
}
