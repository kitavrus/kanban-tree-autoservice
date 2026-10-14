package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/internal/features/auth"
	"github.com/kitavrus/kanban-tree/internal/features/board"
	"github.com/kitavrus/kanban-tree/internal/features/calendar"
	"github.com/kitavrus/kanban-tree/internal/features/catalog"
	"github.com/kitavrus/kanban-tree/internal/features/clients"
	"github.com/kitavrus/kanban-tree/internal/features/executorpayouts"
	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/features/suppliers"
	"github.com/kitavrus/kanban-tree/internal/features/users"
	"github.com/kitavrus/kanban-tree/internal/filestore"
	"github.com/kitavrus/kanban-tree/internal/observability"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/internal/session"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// New собирает Fiber-приложение: middleware, health/ready/metrics, роуты фич.
// Новые фичи регистрируются здесь: feature.Register(protected) — URL по §API-1
// (kebab-сегменты, /api/v1/...), JSON-ключи snake_case; SSR-экраны — через
// ssr.NewRenderer(root, ssr.SharedRoot) + Handlers.Register(app).
func New(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool, rdb *redis.Client) (*fiber.App, error) {
	// StreamRequestBody (фаза 7): тело не буферизуется fasthttp целиком до хендлера —
	// загрузка видео до 500 МБ идёт потоком в FileStore. Лимит «как было» (4 МБ) для всех
	// прочих роутов держит middleware.BodyLimit ниже; предразбор multipart на уровне
	// сервера выключен (иначе любой роут принимал бы файлы во временные до auth/CSRF).
	app := fiber.New(fiber.Config{
		ErrorHandler:                 errorspkg.FiberErrorHandler,
		JSONDecoder:                  strictJSON,
		BodyLimit:                    fiber.DefaultBodyLimit,
		StreamRequestBody:            true,
		DisablePreParseMultipartForm: true,
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(observability.TraceMiddleware("kanban-tree"))
	app.Use(observability.MetricsMiddleware())
	app.Use(middleware.RequestLogger(logger))
	// Лимит тела — ДО CSRF (CSRF читает форму из тела) и до любых хендлеров.
	app.Use(middleware.BodyLimit(fiber.DefaultBodyLimit, board.IsAttachmentUpload, board.AttachmentUploadMaxBody))

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/ready", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			return errorspkg.Internal("db not ready")
		}
		if err := rdb.Ping(ctx).Err(); err != nil {
			return errorspkg.Internal("redis not ready")
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// SSR: общая статика (Bootstrap, app.css, js/) — один маунт на проект (путь — как
	// у /z-scaffold). Шаблонов под корнем статики НЕТ (они в web/ssr/_layout): фильтр
	// по URL обходится кодированием/регистром/`..`, поэтому закрытость — структурная.
	// Cache-Control: no-cache — браузер ревалидирует файл на каждой загрузке страницы
	// (If-Modified-Since → 304 без тела, если не менялся). Без заголовка браузер кэширует
	// эвристически по Last-Modified, и после обновления сервера держит старый JS/CSS.
	app.Use("/assets/ssr-shared", static.New("./web/ssr/_shared", static.Config{
		ModifyResponse: func(c fiber.Ctx) error {
			c.Set(fiber.HeaderCacheControl, "no-cache")
			return nil
		},
	}))

	// CSRF (double-submit) — на все маршруты ниже, включая /api/v1 и /login: API ходит
	// с той же cookie-сессией из браузера.
	app.Use(middleware.CSRF(cfg.Auth.CookieSecure))

	store := session.NewStore(rdb, cfg.Auth.SessionTTL, cfg.Auth.SessionAbsoluteMax)
	authSvc := auth.NewService(pool, rdb, store, cfg.Auth)
	guard := &middleware.Auth{Store: store, Load: authSvc.Principal, Secure: cfg.Auth.CookieSecure, MaxAge: cfg.Auth.SessionAbsoluteMax}

	// board.StockService — источник бейджа «Остатки» в сайдбаре (ADR-004); создаётся
	// раньше ShellBuilder, чтобы WithBadges попал во ВСЕ SSR-экраны разом (общий layout).
	boardColRepo := board.NewColumnRepo(pool)
	boardTaskRepo := board.NewTaskRepo(pool)
	boardEngine := board.NewStockEngine()
	boardStockRepo := board.NewStockRepo(pool)
	boardStockSvc := board.NewStockService(boardStockRepo)
	// Бейдж «Остатки» кэшируется (board/nav_badge_cache.go). risk_count зависит от товаров,
	// заказов, услуг и флагов колонок, поэтому кэш сбрасывается после ЛЮБОГО успешного
	// изменяющего запроса, а не точечно в каждом сервисе. Зарегистрировано до маршрутов —
	// проходит через все них.
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		default:
			if err == nil && c.Response().StatusCode() < fiber.StatusBadRequest {
				boardStockSvc.InvalidateNavBadges()
			}
		}
		return err
	})
	boardEquipmentRepo := board.NewTaskEquipmentRepo()
	boardCardRepo := board.NewCardRepo(pool)
	boardSubtaskRepo := board.NewSubtaskRepo()
	boardSubtaskProductRepo := board.NewSubtaskProductRepo()
	boardSubtaskLogRepo := board.NewSubtaskLogRepo(pool)

	// Пункты сайдбара — только реализованные разделы (растёт по фазам plan.md §6).
	shell := ssr.NewShellBuilder("/", "/stock", "/users", "/catalog", "/clients", "/suppliers", "/executor-payouts", "/calendar", "/deleted").WithBadges(boardStockSvc)

	sharedR, err := ssr.NewRenderer(ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	status := ssr.NewStatusPages(sharedR, shell)
	authR, err := ssr.NewRenderer(auth.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	usersR, err := ssr.NewRenderer(users.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	boardR, err := ssr.NewRenderer(board.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	clientsR, err := ssr.NewRenderer(clients.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	catalogR, err := ssr.NewRenderer(catalog.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	payoutsR, err := ssr.NewRenderer(executorpayouts.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	calendarR, err := ssr.NewRenderer(calendar.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	suppliersR, err := ssr.NewRenderer(suppliers.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}
	deletedR, err := ssr.NewRenderer(softdelete.Root, ssr.SharedRoot)
	if err != nil {
		return nil, err
	}

	api := app.Group("/api/v1")
	auth.Register(app, api, auth.NewHandler(authSvc, guard, authR, shell), guard)
	usersSvc := users.NewService(pool, store)
	users.Register(app, api, guard, users.NewHandler(usersSvc), users.NewSSR(usersSvc, usersR, shell, status))
	clientsSvc := clients.NewService(pool)
	clients.Register(app, api, guard, clients.NewHandler(clientsSvc), clients.NewSSR(clientsSvc, clientsR, shell, status))
	catCatSvc := catalog.NewCategoryService(pool)
	catProdSvc := catalog.NewProductService(pool)
	catSvcSvc := catalog.NewServiceService(pool)
	catalog.Register(app, api, guard, catalog.NewCategoryHandler(catCatSvc), catalog.NewProductHandler(catProdSvc), catalog.NewServiceHandler(catSvcSvc),
		catalog.NewSSR(catCatSvc, catProdSvc, catSvcSvc, pool, catalogR, shell, status))
	// Справочник «Поставщики» (suppliers DS-007): только admin.
	suppliersSvc := suppliers.NewService(pool)
	suppliers.Register(app, api, guard, suppliers.NewHandler(suppliersSvc),
		suppliers.NewSSR(suppliersR, shell, status))
	// Выплаты исполнителям (D62–D70): отчёт и массовая отметка — и admin, и executor.
	executorpayouts.Register(app, api, guard, executorpayouts.NewHandler(executorpayouts.NewService(pool)),
		executorpayouts.NewSSR(payoutsR, shell))
	// Календарь загрузки сервиса: смотрят все, вместимость (5–12 машин) меняет admin.
	calendarSvc := calendar.NewService(pool)
	calendar.Register(app, api, guard, calendar.NewHandler(calendarSvc), calendar.NewSSR(calendarSvc, calendarR, shell))
	// Раздел «Удалённые» (soft-delete-core DS-011): только admin. Источники видов подключают
	// части мастер-спеки soft-delete; часть 1 — заказы.
	deletedSvc := softdelete.NewService().
		Register(softdelete.KindTasks, softdelete.NewTasksSource(pool)).
		// Часть 3 (soft-delete-board-columns) — колонки доски.
		Register(softdelete.KindColumns, softdelete.NewColumnsSource(pool)).
		// Часть 4 (soft-delete-clients) — клиенты и единицы оборудования.
		Register(softdelete.KindClients, softdelete.NewClientsSource(pool)).
		Register(softdelete.KindEquipment, softdelete.NewEquipmentSource(pool)).
		// Часть 5 (soft-delete-catalog) — товары, услуги, категории каталога.
		Register(softdelete.KindProducts, softdelete.NewProductsSource(pool)).
		Register(softdelete.KindServices, softdelete.NewServicesSource(pool)).
		Register(softdelete.KindCatalogCategories, softdelete.NewCatalogCategoriesSource(pool)).
		// Часть 6 (soft-delete-suppliers) — поставщики.
		Register(softdelete.KindSuppliers, softdelete.NewSuppliersSource(pool))
	softdelete.Register(app, api, guard, softdelete.NewHandler(deletedSvc), softdelete.NewSSR(deletedR, shell, status))

	// board — доска заказов занимает корень «/» (ADR-100, снимает home) + «/stock» +
	// печать актов (фаза 04) — один CardService на JSON API и SSR-печать (Re-fetch-after-write
	// не задействован печатью, но Card() — общий контур доступа executor/admin, D19).
	// Вложения заказов (фаза 7): локальный диск UPLOADS_DIR за интерфейсом FileStore.
	files, err := filestore.NewLocal(cfg.UploadsDir)
	if err != nil {
		return nil, err
	}
	boardCardSvc := board.NewCardService(pool, boardTaskRepo, boardCardRepo, boardEquipmentRepo, boardEngine,
		boardSubtaskRepo, boardSubtaskProductRepo, boardSubtaskLogRepo).WithFiles(files)
	boardColumns := board.NewColumnsHandler(board.NewColumnService(pool, boardColRepo, boardEngine))
	board.Register(app, api, guard, &board.Handlers{
		Columns: boardColumns,
		Tasks: board.NewTasksHandler(board.NewTaskService(pool, boardTaskRepo, boardEngine, boardEquipmentRepo).WithFiles(files)).
			// Служебная уборка вида columns (soft-delete-board-columns DS-027).
			WithPurger(string(softdelete.KindColumns), boardColumns.Purge).
			// Служебная уборка видов clients и equipment (soft-delete-clients DS-027).
			WithPurger(string(softdelete.KindClients), clientsSvc.PurgeClient).
			WithPurger(string(softdelete.KindEquipment), clientsSvc.PurgeEquipment).
			// Служебная уборка видов каталога (soft-delete-catalog DS-027).
			WithPurger(string(softdelete.KindProducts), catProdSvc.Purge).
			WithPurger(string(softdelete.KindServices), catSvcSvc.Purge).
			WithPurger(string(softdelete.KindCatalogCategories), catCatSvc.Purge).
			// Служебная уборка вида поставщик (soft-delete-suppliers DS-027).
			WithPurger(string(softdelete.KindSuppliers), suppliersSvc.Purge),
		Assignees:   board.NewAssigneesHandler(pool),
		Stock:       board.NewStockHandler(boardStockSvc),
		Card:        board.NewCardHandler(boardCardSvc),
		Attachments: board.NewAttachmentsHandler(boardCardSvc),
		// Служебная уборка автотестов (soft-delete-core DS-027): только TEST_PURGE_ENABLED=true.
		PurgeEnabled: cfg.TestPurgeEnabled,
	}, board.NewSSR(boardR, shell, status, cfg.Firm, boardCardSvc))

	return app, nil
}
