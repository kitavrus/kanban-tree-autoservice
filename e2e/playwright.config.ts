import { defineConfig, devices } from '@playwright/test';

// Стенд поднимается отдельно (make local-up; порт — PORT из .env, по умолчанию 8080):
// webServer здесь намеренно не описан — серверу нужны PG/Redis, Playwright их не поднимает.
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:8080';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  // Watch-gate (§TD-5, PWSLOWMO): замедление N мс на действие — длинным формам нужен
  // больший таймаут теста, иначе он истекает от slow-mo, а не от дефекта.
  timeout: process.env.PWSLOWMO ? 180_000 : 30_000,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    locale: 'ru-RU',
    // Замедление «для глаз» (CLAUDE.md §TD-5 watch-gate): PWSLOWMO=600 npm --prefix e2e test.
    launchOptions: { slowMo: Number(process.env.PWSLOWMO ?? 0) },
  },
  // CLAUDE.md §TD-3: гоняем только chromium и только headed (флаг --headed в npm-скриптах).
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
