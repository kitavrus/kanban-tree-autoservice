---
stage: 3-design
feature: phase-3-clients
tier: M
---
# Design Errors: phase-3-clients

Формат — `pkg/errorspkg` без изменений: `{code,message,details?}`, 400 `VALIDATION` с `details.fields[{name,message}]` (message — код, SSR переводит в текст). Коды — строковые (как `LOGIN_TAKEN`), числовых supportMessage в проекте нет.

## Sentinel Errors
`internal/features/clients/service.go`:
```go
var (
	ErrClientNotFound        = errorspkg.New("CLIENT_NOT_FOUND", http.StatusNotFound, "Клиент не найден")
	ErrEquipmentNotFound     = errorspkg.New("EQUIPMENT_NOT_FOUND", http.StatusNotFound, "Оборудование не найдено")
	ErrClientVersionConflict = errorspkg.New("CLIENT_VERSION_CONFLICT", http.StatusConflict,
		"Карточку уже изменил другой пользователь — ваши правки не сохранены")
)
```
`internal/features/equipmenttypes/service.go`:
```go
var (
	ErrTypeNotFound          = errorspkg.New("EQUIPMENT_TYPE_NOT_FOUND", http.StatusNotFound, "Тип оборудования не найден")
	ErrTypeNameTaken         = errorspkg.New("EQUIPMENT_TYPE_NAME_TAKEN", http.StatusConflict, "Такой тип уже есть")
	ErrTypeNameArchived = errorspkg.New("EQUIPMENT_TYPE_NAME_ARCHIVED", http.StatusConflict,
		"Такой тип уже есть в архиве — верните его из архива")                 // ADR-007: отдельный код → клиент показывает CTA «Вернуть из архива» по code, не по тексту
	ErrTypeInUse = errorspkg.New("EQUIPMENT_TYPE_IN_USE", http.StatusConflict,
		"Тип используется в оборудовании клиентов — удалить нельзя, отправьте его в архив")
)
```
Repo-уровень (неэкспортируемые, маппинг pg → домен в repo, `errors.Is` в сервисе — паттерн users): `ErrNotFound = pgx.ErrNoRows`, `errVersionConflict`, `errNameTaken`, `errNameArchived`, `errInUse` (23503), `errClientGone` (23503 на `equipment.client_id`).

НЕ вводятся (фаза 5, Tech Spec D26): `CLIENT_HAS_ORDERS`, `EQUIPMENT_IN_ORDERS`.

## Codes Fields
Коды `details.fields[].message` (константы в `validate.go` каждой фичи; `required/too_long/invalid_format/invalid` — как users):
| Код | Смысл | Поля |
|---|---|---|
| `required` | пусто после trim | `company_name`, `phones`, `phones[i].phone`, `model`, `type_id`, `name`, `version`, `is_active` |
| `too_long` | превышение rune-лимита | все строковые, `specs[i].k`, `specs[i].v`, `spec_keys[i]` |
| `too_many` | превышение числа строк | `phones` (>20), `specs` (>50), `spec_keys` (>50 после дедупа) |
| `invalid_format` | телефон без цифр | `phones[i].phone` |
| `invalid` | битый uuid | `type_id` |
| `not_found` | тип не существует | `type_id` |
| `archived` | тип в архиве (создание/смена) | `type_id` |
| `key_required` | значение без названия характеристики | `specs[i].k` |
| `invalid_text` | NUL-байт или невалидный UTF-8 (code review fix-loop #1, §TD-4: мусор → 400, не 500 от Postgres 22021) | все пользовательские строки clients / equipment / equipment-types |

Индекс `i` — позиция в НОРМАЛИЗОВАННОМ списке (пустые строки уже отброшены); SSR перерисовывает форму нормализованным списком → индексы совпадают со строками формы.

## Error Mapping
### API (`errorspkg.FiberErrorHandler`, без изменений)
| Ситуация | HTTP | code |
|---|---|---|
| невалидный JSON / неизвестные поля (`strictJSON`) | 400 | `BAD_REQUEST`, message «Некорректное тело запроса» (новые хендлеры фазы 3; `users` — as-is «invalid body», не переписывается молча) |
| ошибки полей | 400 | `VALIDATION` + `details.fields` |
| битый/несуществующий `:id` | 404 | `CLIENT_NOT_FOUND` / `EQUIPMENT_NOT_FOUND` / `EQUIPMENT_TYPE_NOT_FOUND` |
| POST equipment, клиента нет / удалён параллельно | 404 | `CLIENT_NOT_FOUND` |
| устаревший `version` | 409 | `CLIENT_VERSION_CONFLICT` |
| дубль имени активного типа | 409 | `EQUIPMENT_TYPE_NAME_TAKEN` |
| дубль имени архивного типа (ADR-007) | 409 | `EQUIPMENT_TYPE_NAME_ARCHIVED` — клиент находит тип `GET /equipment-types?include_archived=true&q=<name>` и предлагает `PUT … is_active:true` |
| DELETE используемого типа | 409 | `EQUIPMENT_TYPE_IN_USE` |
| executor на admin-мутации | 403 | `FORBIDDEN` (`RequireAdmin(nil)`) |
| без сессии | 401 | `UNAUTHORIZED` (`RequireAPI`) |

### SSR (`formError` в `clients/ssr.go`, `equipmenttypes/ssr.go` — эталон `users/ssr.go`)
| Ошибка | Статус | Отображение |
|---|---|---|
| `VALIDATION` | 422 | текст у поля (`fieldText`), ввод сохранён |
| `ErrClientVersionConflict` | 409 | `vm.Error` = message + `vm.Conflict` (ссылка «Открыть актуальную версию», ADR-005) |
| `ErrTypeNameTaken` | 409 | текст у поля `name` |
| `ErrTypeNameArchived` | 409 | текст у поля `name` + ссылка «Показать архивные» (`/catalog/equipment-types?archived=1`) |
| `ErrTypeInUse` | 409 | форма типа: `vm.Error` + видимая кнопка «В архив» |
| `*NotFound` | 404 | `status.NotFound` |
| прочее ≥500 | 500 | пробрасывается (`FiberErrorHandler`) |

### fieldText (SSR, тексты)
| Код | Текст |
|---|---|
| `required` | «Обязательное поле»; `phones` — «Укажите хотя бы один телефон»; `type_id` — «Выберите тип» |
| `too_long` | «Слишком длинное значение» |
| `too_many` | `phones` — «Не больше 20 телефонов»; `specs`/`spec_keys` — «Не больше 50 характеристик» |
| `invalid_format` | «Номер должен содержать цифры» |
| `invalid` / `not_found` | «Тип не найден» |
| `archived` | «Тип в архиве — выберите другой» |
| `key_required` | «Укажите характеристику» |
| `invalid_text` | «Недопустимое значение» |
