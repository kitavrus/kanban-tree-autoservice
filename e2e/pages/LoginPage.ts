import { expect, type Locator, type Page } from '@playwright/test';

// POM страницы входа (web/ssr/auth/templates/pages/login.html).
export class LoginPage {
  readonly form: Locator;
  readonly login: Locator;
  readonly password: Locator;
  readonly submit: Locator;
  readonly error: Locator;

  constructor(readonly page: Page) {
    this.form = page.getByTestId('login-form');
    this.login = page.getByLabel('Логин');
    this.password = page.getByLabel('Пароль');
    this.submit = page.getByRole('button', { name: 'Войти' });
    this.error = page.getByTestId('login-error');
  }

  async goto(redirect?: string) {
    await this.page.goto(redirect ? `/login?redirect=${encodeURIComponent(redirect)}` : '/login');
    await expect(this.form).toBeVisible();
  }

  async signIn(login: string, password: string) {
    await this.login.fill(login);
    await this.password.fill(password);
    await this.submit.click();
  }
}
