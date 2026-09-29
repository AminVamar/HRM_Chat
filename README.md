# HRM Chat

Чат для Системы HRM — бэкенд на Go (PostgreSQL, WebSocket, шифрование сообщений AES-256-GCM).

## Запуск через Docker

```bash
git clone https://github.com/AminVamar/HRM_Chat.git
cd HRM_Chat
cp .env.example .env   # заполнить значения
docker compose up -d --build
```

Обязательно задайте в `.env`:

| Переменная | Описание |
|---|---|
| `DB_PASSWORD` | пароль PostgreSQL |
| `CORS_ORIGINS` | адреса фронтенда через запятую, например `https://hrm.example.com` |
| `SWAGGER_HOST` | адрес сервера для Swagger, например `192.168.1.10:8080` |
| `HOST_PORT` | порт API на сервере (по умолчанию `8080`) |

Миграции базы применяются автоматически при старте.

Логи и остановка:

```bash
docker compose logs -f app
docker compose down
```

## Для фронтенда

- Базовый URL API: `http://<сервер>:8080/api`
- Swagger: `http://<сервер>:8080/swagger/index.html`
- Авторизация: заголовок `Login: <username>`
- WebSocket: `ws://<сервер>:8080/api/ws?login=<username>`
- CORS: адрес фронтенда должен быть указан в `CORS_ORIGINS`

При первом запросе пользователь автоматически создаётся по `username`. При следующих
запросах используется тот же профиль и ID. Пробелы по краям имени удаляются, регистр
не учитывается. Email для входа не используется; у нового пользователя поле `email`
в ответе будет пустой строкой. Имя обязательно, максимальная длина — 255 символов.

HTTP API также принимает `?login=<username>`, если заголовок `Login` отсутствует.
Пустое имя возвращает 401, некорректное — 400, ошибка базы — 500.

При обновлении существующей базы email становится необязательным. Если есть имена,
отличающиеся только регистром, миграция остановится с ошибкой: такие записи нужно
разобрать вручную, сохранив нужные связи и переписку, затем повторить запуск.

Проверка после запуска (PowerShell):

```powershell
docker compose up -d --build
docker compose ps
curl.exe -s -H "Login: test_user" http://localhost:8080/api/users/me
curl.exe -s -H "Login: TEST_USER" http://localhost:8080/api/users/me
docker compose logs --tail=100 app
```

Оба запроса должны вернуть одинаковый `id`. Swagger доступен по адресу
`http://localhost:8080/swagger/index.html`: в Authorize укажите имя пользователя,
затем выполните `GET /users/me`. Если `HOST_PORT` изменён, используйте этот порт.