import type { Page } from '@playwright/test';

// TS-F5 — фикстура времени (docs/features/user-timezone/draft-spec.md §11): один момент
// 2026-10-06T19:30:00Z — это 00:30 7 октября по Алматы и 22:30 6 октября по Москве: и время,
// и КАЛЕНДАРНАЯ ДАТА у двух пользователей разные. Пояс браузера (TZ_BROWSER) намеренно
// третий: тест падает, если экран форматирует время «по браузеру», а не по поясу пользователя.
//
// pg удалён из зависимостей e2e (ADR-101), а created_at/…_at сервер ставит сам (now()),
// поэтому запись с точным моментом в E2E заводится так: данные — настоящие, через боевое
// API; момент «прикалывается» в JSON-ответах GET (pinFixtureMoment) — форматирует его уже
// настоящий код страницы. SSR-экраны (история оборудования, печать) приколоть нельзя — для
// них проверяется сдвиг между двумя пользователями (minutesBetween); точный момент в БД для
// SSR покрыт Go live-тестами (internal/features/{clients,board}/…TS041…).

export const TZ_ALMATY = 'Asia/Almaty';
export const TZ_MOSCOW = 'Europe/Moscow';
/** Пояс браузера в спеках user-timezone — не совпадает ни с одним поясом пользователя. */
export const TZ_BROWSER = 'America/New_York';

export const TZ_LABEL = {
  [TZ_ALMATY]: 'Алматы (UTC+5)',
  [TZ_MOSCOW]: 'Москва (UTC+3)',
} as const;

export type ZoneView = {
  zone: string;
  /** «DD.MM.YYYY» момента фикстуры в поясе. */
  date: string;
  /** «DD.MM.YYYY HH:MM» момента фикстуры в поясе. */
  dateTime: string;
  /** Календарная дата «сегодня» (YYYY-MM-DD), если «сейчас» = момент фикстуры. */
  today: string;
};

export const TZ_FIXTURE = {
  moment: '2026-10-06T19:30:00Z',
  almaty: { zone: TZ_ALMATY, date: '07.10.2026', dateTime: '07.10.2026 00:30', today: '2026-10-07' } satisfies ZoneView,
  moscow: { zone: TZ_MOSCOW, date: '06.10.2026', dateTime: '06.10.2026 22:30', today: '2026-10-06' } satisfies ZoneView,
  /** Алматы (UTC+5) впереди Москвы (UTC+3) на 2 часа. */
  almatyAheadMinutes: 120,
} as const;

type Json = null | boolean | number | string | Json[] | { [key: string]: Json };

function pin(value: Json): Json {
  if (Array.isArray(value)) return value.map(pin);
  if (value !== null && typeof value === 'object') {
    const out: { [key: string]: Json } = {};
    for (const [key, v] of Object.entries(value)) {
      out[key] = key.endsWith('_at') && typeof v === 'string' ? TZ_FIXTURE.moment : pin(v);
    }
    return out;
  }
  return value;
}

/**
 * Во всех JSON-ответах GET /api/v1/** страницы заменяет значения полей `*_at` на момент
 * фикстуры TS-F5. Остальное тело, статус и заголовки — настоящие; мутации не трогаются.
 */
export async function pinFixtureMoment(page: Page): Promise<void> {
  await page.route('**/api/v1/**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    try {
      const response = await route.fetch();
      const type = response.headers()['content-type'] ?? '';
      if (!response.ok() || !type.includes('application/json')) {
        await route.fulfill({ response });
        return;
      }
      await route.fulfill({ response, json: pin((await response.json()) as Json) });
    } catch {
      // Страница ушла/закрылась, пока запрос летел (фоновые запросы доски) — отменяем.
      await route.abort().catch(() => undefined);
    }
  });
}

/** «DD.MM.YYYY HH:MM» → минуты условной шкалы (поле за полем, без пояса браузера). */
export function wallMinutes(text: string): number {
  const m = /(\d{2})\.(\d{2})\.(\d{4}) (\d{2}):(\d{2})/.exec(text);
  if (!m) throw new Error(`не дата-время «DD.MM.YYYY HH:MM»: «${text}»`);
  const [, dd, mm, yyyy, hh, mi] = m;
  return Date.UTC(Number(yyyy), Number(mm) - 1, Number(dd), Number(hh), Number(mi)) / 60_000;
}

/** На сколько минут показанное время `a` впереди показанного времени `b`. */
export function minutesBetween(a: string, b: string): number {
  return wallMinutes(a) - wallMinutes(b);
}
