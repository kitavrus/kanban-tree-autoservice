-- Откат часового пояса пользователя: настройки поясов теряются (CHECK уходит вместе с колонкой).
ALTER TABLE users DROP COLUMN IF EXISTS timezone;
