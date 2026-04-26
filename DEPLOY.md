# Руководство по развёртыванию в продакшене

Пошаговые инструкции по развёртыванию на чистом сервере Ubuntu/Debian.

---

## Требования

- Сервер с Ubuntu 22.04+ или Debian 12+, минимум 2 ГБ RAM
- Домен с A-записью, указывающей на IP сервера (DNS должен распространиться до развёртывания)
- Открытые порты 80 и 443 в брандмауэре сервера и вышестоящем брандмауэре/группе безопасности

---

## Шаг 1 — Установить Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker

# Убедиться, что Compose V2 доступен (поставляется с Docker Engine 24+)
docker compose version
# Ожидаемый вывод: Docker Compose version v2.x.x
```

> **Важно:** НЕ устанавливайте устаревший бинарник `docker-compose`. Все команды в этом руководстве используют `docker compose` (с пробелом, без дефиса).

---

## Шаг 2 — Клонировать репозиторий

```bash
cd /opt
git clone <url-репозитория> vpnplatform
cd vpnplatform
```

---

## Шаг 3 — Создать и заполнить `.env`

```bash
cp .env.example .env
nano .env
```

Сгенерировать надёжные секреты:

```bash
# Для POSTGRES_PASSWORD и REDIS_PASSWORD (64-символьная hex-строка)
openssl rand -hex 32

# Для JWT_SECRET (не менее 32 символов; использовать 128-символьный hex)
openssl rand -hex 64
```

Заполните каждую переменную в `.env`. В таблице ниже указано, что нужно:

| Переменная | Откуда взять |
|---|---|
| `DOMAIN` | Ваш домен, например `yourdomain.com` |
| `ADMIN_EMAIL` | Ваш email — сюда приходят уведомления об истечении сертификата Let's Encrypt |
| `POSTGRES_PASSWORD` | `openssl rand -hex 32` |
| `REDIS_PASSWORD` | `openssl rand -hex 32` |
| `JWT_SECRET` | `openssl rand -hex 64` |
| `PLATEGA_MERCHANT_ID` | Личный кабинет мерчанта Platega |
| `PLATEGA_SECRET` | Личный кабинет мерчанта Platega |
| `PLATEGA_CALLBACK_URL` | `https://yourdomain.com/webhooks/platega` ← **без префикса `/api/`** |
| `REMNA_BASE_URL` | URL вашей панели администратора Remnawave |
| `REMNA_API_KEY` | Настройки API Remnawave |
| `TELEGRAM_TOKEN` | [@BotFather](https://t.me/BotFather) → /newbot |
| `TELEGRAM_ADMIN_ID` | Ваш числовой Telegram user ID (используйте [@userinfobot](https://t.me/userinfobot)) |
| `TELEGRAM_BOT_USERNAME` | Имя вашего бота без `@` |
| `WEBAPP_URL` | `https://yourdomain.com` |
| `ADMIN_BOOTSTRAP_TOKEN` | Одноразовый токен (≥24 символа, `openssl rand -hex 32`). Первый зарегистрированный пользователь, передавший этот токен в поле `bootstrap_token`, становится админом. После создания первого админа очистите переменную; следующие админы — через `UPDATE users SET is_admin=TRUE WHERE username='foo'`. Заменяет старый небезопасный `ADMIN_LOGIN`. |
| `JWT_ACCESS_TTL_HOURS` | Срок жизни access-токена в часах. Допустимый диапазон 1..24, по умолчанию 1. |
| `ALLOWED_ORIGINS` | `https://yourdomain.com` — сервер откажется стартовать с `*` в продакшене (CORS-уязвимость). |
| `ALLOWED_RETURN_HOSTS` | Хосты (через запятую), допустимые в `return_url` платежей. Пустое значение = `{DOMAIN, www.DOMAIN}`. Закрывает open-redirect через ссылку Platega. |
| `PLATEGA_SECRET_PREV` | Предыдущий секрет Platega на время ротации (24–48 ч). Очистите после миграции. |

> **Предупреждение о PLATEGA_CALLBACK_URL:** Путь должен быть `/webhooks/platega` (не `/api/webhooks/platega`). Caddy направляет `/webhooks/*` напрямую к Go-бэкенду, который регистрирует хэндлер именно по пути `/webhooks/platega`. Добавление префикса `/api/` приведёт к 404 на каждый входящий платёж и все подписки зависнут в ожидании.

---

## Шаг 4 — Открыть порты

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

---

## Шаг 5 — Развернуть

```bash
docker compose up -d --build
```

При первом запуске Docker скачивает базовые образы, компилирует Go-бинарники, а Caddy запрашивает TLS-сертификат у Let's Encrypt. Подождите 5–10 минут.

---

## Шаг 6 — Проверить

```bash
# Все сервисы должны быть "running" или "healthy"
docker compose ps

# Логи Caddy — ждите "certificate obtained successfully"
docker compose logs caddy --tail 50

# Эндпоинт проверки работоспособности API (должен вернуть {"status":"ok",...})
curl https://yourdomain.com/api/health

# Открыть в браузере — обязателен зелёный замок
https://yourdomain.com
```

---

## Типовые операции

### Статус и логи

```bash
# Все контейнеры
docker compose ps

# Live-логи (Ctrl-C для остановки)
docker compose logs -f

# Один сервис
docker compose logs backend --tail 100
docker compose logs caddy   --tail 50
docker compose logs bot     --tail 50

# Фильтр по ошибкам
docker compose logs | grep -i error
```

### Управление сервисами

```bash
# Перезапустить один сервис
docker compose restart backend

# Остановить всё
docker compose down

# Полная пересборка и перезапуск
docker compose up -d --build

# Обновить код без простоя
git pull
docker compose up -d --build
```

### Shell в контейнерах

```bash
# Go-бэкенд
docker exec -it vpn_backend sh

# Консоль PostgreSQL
docker exec -it vpn_postgres psql -U vpn -d vpnplatform

# Redis CLI (читает пароль из .env)
REDIS_PASS=$(grep ^REDIS_PASSWORD .env | cut -d= -f2)
docker exec -it vpn_redis redis-cli --pass "$REDIS_PASS"
```

---

## Резервное копирование базы данных

Резервные копии хранятся в именованном томе Docker `pgbackups`, управляемом контейнером `pg_backup` (ежедневно в полночь UTC, хранятся последние 7).

```bash
# Список существующих резервных копий
docker exec vpn_pg_backup ls -lh /backups/

# Ручной сжатый дамп
docker exec vpn_postgres sh -c \
  'PGPASSWORD=$POSTGRES_PASSWORD pg_dump -U vpn vpnplatform' \
  | gzip > "backup-$(date +%Y%m%d-%H%M%S).sql.gz"

# Восстановление из файла дампа
gunzip -c backup-20260101-120000.sql.gz \
  | docker exec -i vpn_postgres psql -U vpn -d vpnplatform

# Копия тома бэкапов на локальную машину
docker run --rm \
  -v vpnplatform_pgbackups:/data \
  alpine tar czf - /data \
  | ssh user@server "cat > pgbackups-$(date +%Y%m%d).tar.gz"
```

---

## SSL-сертификаты

Caddy полностью автоматически управляет жизненным циклом сертификатов:

- Запрашивает у Let's Encrypt при первом запуске.
- Обновляет за 30 дней до истечения срока.
- Хранится в томе `caddy_data` — сохраняется при перезапуске и пересборке контейнеров.

```bash
# Проверить статус сертификата
docker compose logs caddy | grep -i "certificate\|tls\|acme"

# Проверить снаружи
curl -I https://yourdomain.com
```

**Если сертификат не выдан:**

```bash
# 1. Проверить, что DNS указывает на этот сервер
nslookup yourdomain.com

# 2. Проверить, что порты открыты
sudo ufw status
sudo ss -tlnp | grep -E ':80|:443'

# 3. Лимит запросов Let's Encrypt — проверить логи Caddy на детали ошибки
docker compose logs caddy | grep -i "rate limit"

# 4. Перезапустить Caddy
docker compose restart caddy
```

---

## Мониторинг

```bash
# Использование ресурсов (в реальном времени)
docker stats

# Место на диске, занятое томами и образами Docker
docker system df

# Размер базы данных PostgreSQL
docker exec vpn_postgres psql -U vpn -d vpnplatform -c \
  "SELECT pg_size_pretty(pg_database_size('vpnplatform'));"

# Использование памяти Redis
REDIS_PASS=$(grep ^REDIS_PASSWORD .env | cut -d= -f2)
docker exec vpn_redis redis-cli --pass "$REDIS_PASS" info memory | grep used_memory_human
```

---

## Устранение неполадок

### 502 Bad Gateway

```bash
docker compose ps                    # проверить, что все сервисы запущены
docker compose logs backend --tail 50
docker compose logs frontend --tail 50
docker compose restart backend
```

### Ошибка подключения к базе данных

```bash
docker compose logs postgres --tail 30
docker exec vpn_postgres pg_isready  # должно вывести "accepting connections"
grep POSTGRES .env                   # убедиться, что учётные данные совпадают
docker compose restart postgres
```

### Бот не отвечает

```bash
docker compose logs bot --tail 50
grep TELEGRAM_TOKEN .env             # убедиться, что токен установлен
docker exec vpn_bot sh -c 'wget -qO- https://api.telegram.org 2>&1 | head -5'
docker compose restart bot
```

### Платежи зависли в ожидании

Проверьте `PLATEGA_CALLBACK_URL` в `.env`:

```bash
grep PLATEGA_CALLBACK_URL .env
# Правильно:   https://yourdomain.com/webhooks/platega
# Неправильно: https://yourdomain.com/api/webhooks/platega  ← вызывает 404
```

---

## Обновления

```bash
# Получить последний код и пересобрать изменённые контейнеры
git pull
docker compose up -d --build

# Обновить базовые образы (postgres, redis, caddy, nginx)
docker compose pull
docker compose up -d
```

---

## Структура файлов

```
/opt/vpnplatform/
├── .env                  Секреты окружения — НЕ в git
├── .env.example          Зафиксированный шаблон (справочник по всем переменным)
├── docker-compose.yml    Все определения сервисов
├── Caddyfile             Конфиг HTTPS обратного прокси
├── cmd/                  Точки входа Go (api, bot, worker)
├── frontend/             React SPA (Vite + Tailwind)
├── deployments/          Dockerfile-ы для Go-сервисов
├── internal/             Код сервисов, хэндлеров и репозиториев Go
└── migrations/           SQL-миграции схемы
```

Данные резервных копий хранятся в томе Docker `pgbackups` (не как директория в папке проекта).