-- Часовой пояс пользователя (DS-031, DS-033): имя IANA; время хранится в UTC (timestamptz),
-- пояс влияет только на показ. Существующие пользователи получают Алматы (DEFAULT).
-- Распознаваемость имени проверяет сервис (time.LoadLocation); в БД — только длина.
ALTER TABLE users
    ADD COLUMN timezone text NOT NULL DEFAULT 'Asia/Almaty',
    ADD CONSTRAINT users_timezone_len_chk CHECK (char_length(timezone) BETWEEN 1 AND 64);
