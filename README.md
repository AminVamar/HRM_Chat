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
- Авторизация: заголовок `Login: <username или email>`
- WebSocket: `ws://<сервер>:8080/api/ws?login=<username>`
- CORS: адрес фронтенда должен быть указан в `CORS_ORIGINS`
