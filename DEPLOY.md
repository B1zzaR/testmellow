# 🚀 VPN Platform - Production Deployment

Полная инструкция по развертыванию на сервере за 5 минут.

---

## 📋 Требования

- **Домен** с DNS, указывающий на ваш сервер (например `mellowpn.space`)
- **Сервер** с Ubuntu/Debian и 2GB+ RAM
- **Docker** + Docker Compose
- **Открытые порты**: 80 (HTTP), 443 (HTTPS)

---

## 🚀 Быстрый старт (5 минут)

### 1️⃣ На сервере - подготовка

```bash
# SSH на сервер
ssh user@your-server-ip
cd /opt

# Установить Docker (если нет)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker

# Установить Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" \
  -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Клонировать проект
git clone YOUR_REPO_URL vpnplatform
cd vpnplatform
```

### 2️⃣ Настроить переменные (2 минуты)

```bash
# .env уже существует, просто отредактировать его
nano .env
```

**Убедиться что установлены:**
- `DOMAIN=mellowpn.space` → ваш домен
- `ADMIN_EMAIL=admin@mellowpn.space` → ваш email
- `POSTGRES_PASSWORD` → криптографически стойкий пароль (20+ символов)
- `REDIS_PASSWORD` → криптографически стойкий пароль
- `JWT_SECRET` → случайная строка
- `PLATEGA_*` → данные платежного провайдера
- `REMNA_*` → данные VPN провайдера
- `TELEGRAM_TOKEN` → токен бота

Справка по генерации паролей:
```bash
# Генерировать безопасный пароль (20 символов)
openssl rand -base64 20

# Генерировать JWT secret
openssl rand -hex 32
```

### 3️⃣ Открыть firewall

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### 4️⃣ Запустить развертывание (1 минута)

```bash
# Развернуть ВСЕ сервисы
docker-compose up -d --build

# Первый запуск: 5-10 минут (загрузка образов + получение SSL сертификата)
```

### 5️⃣ Проверить статус

```bash
# Все контейнеры запущены?
docker-compose ps

# Все должны быть "running" ✓

# Логи Caddy (HTTPS)
docker-compose logs caddy --tail 30

# Когда видишь "certificate obtained successfully" - готово!
```

### 6️⃣ Проверить в браузере

```
https://mellowpn.space
```

Должен быть **зеленый замок** 🔒 и работающая платформа ✅

---

## 📊 Основные команды

### Статус и логи

```bash
# Статус всех контейнеров
docker-compose ps

# Live логи (Ctrl+C для выхода)
docker-compose logs -f

# Логи конкретного сервиса
docker-compose logs backend --tail 50
docker-compose logs caddy --tail 50
docker-compose logs bot --tail 50

# Логи с фильтром
docker-compose logs | grep -i error
docker-compose logs caddy | grep certificate
```

### Управление

```bash
# Остановить все
docker-compose down

# Перезагрузить конкретный сервис
docker-compose restart backend
docker-compose restart caddy

# Пересоздать с пересборкой
docker-compose up -d --build

# Обновить код и перезагрузить
git pull
docker-compose up -d --build
```

### SSH в контейнеры

```bash
# Backend (Go приложение)
docker exec -it vpn_backend sh

# Фронтенд (веб-сервер)
docker exec -it vpn_frontend sh

# База данных
docker exec -it vpn_postgres psql -U vpn -d vpnplatform

# Redis
docker exec -it vpn_redis redis-cli -a $REDIS_PASSWORD
```

---

## 🔐 SSL Сертификаты

### ✅ Автоматическое управление

- **Caddy автоматически** получает сертификат от Let's Encrypt
- **Автоматически** обновляет за 30 дней до истечения
- **Ничего не нужно делать** - все работает сам

### 🔍 Проверить сертификат

```bash
# Список сертификатов
docker exec vpn_caddy caddy list-certificates

# Дата истечения
docker exec vpn_caddy caddy list-certificates | jq '.certificates[] | {domain: .subjects[0], expires, remaining}'

# Проверить в браузере
curl -I https://mellowpn.space
```

### 🚨 Если сертификат не получен

```bash
# 1. Проверить логи
docker-compose logs caddy

# 2. Проверить DNS
nslookup mellowpn.space
# Должен вернуть IP вашего сервера

# 3. Проверить firewall
sudo ufw status
sudo netstat -tulpn | grep -E ':(80|443)'

# Частые проблемы:
# - DNS еще не распространился (ждать 30 мин)
# - Ports закрыты провайдером
# - Let's Encrypt rate limit (ждать 1 час)

# 4. Перезагрузить Caddy
docker-compose restart caddy
```

---

## 💾 Резервные копии

### Автоматические бэкапы

Контейнер `pg_backup` **ежедневно** создает архивы БД:

```bash
# Посмотреть существующие бэкапы
ls -la pgbackups/

# Скачать на локальную машину
scp -r user@server:/opt/vpnplatform/pgbackups ~/backups/
```

### Ручное создание бэкапа

```bash
# Полный дамп БД
docker exec vpn_postgres pg_dump -U vpn vpnplatform > backup-$(date +%Y%m%d-%H%M%S).sql

# Архивированный дамп
docker exec vpn_postgres pg_dump -U vpn vpnplatform | gzip > backup-$(date +%Y%m%d-%H%M%S).sql.gz

# Восстановить из бэкапа
docker exec -i vpn_postgres psql -U vpn vpnplatform < backup-20240101.sql
```

---

## 📈 Мониторинг

### Использование ресурсов

```bash
# Live статистика
docker stats

# С интервалом
watch -n 5 'docker stats --no-stream'

# Использование диска
df -h
du -sh /opt/vpnplatform/*

# Объем базы данных
docker exec vpn_postgres du -sh /var/lib/postgresql/data
```

### Здоровье сервисов

```bash
# PostgreSQL
docker exec vpn_postgres pg_isready

# Redis
docker exec vpn_redis redis-cli ping

# Frontend доступен
curl -I http://localhost

# Backend доступен
curl -I http://localhost:8080/health

# HTTPS работает
curl -I https://mellowpn.space
```

---

## 🆘 Решение проблем

### ❌ "Site not found" или 502 Bad Gateway

```bash
# 1. Проверить что все контейнеры запущены
docker-compose ps

# 2. Проверить логи фронтенда
docker-compose logs frontend

# 3. Проверить логи бэкенда
docker-compose logs backend

# 4. Перезагрузить
docker-compose restart

# 5. Полная пересборка
docker-compose down
docker-compose up -d --build
```

### ❌ Database connection error

```bash
# Проверить PostgreSQL
docker-compose logs postgres

# Проверить что база здорова
docker exec vpn_postgres pg_isready

# Перезагрузить БД
docker-compose restart postgres

# Проверить credentials в .env
grep POSTGRES .env
```

### ❌ Redis connection refused

```bash
# Логи Redis
docker-compose logs redis

# Проверить Redis
docker exec vpn_redis redis-cli ping

# Перезагрузить Redis
docker-compose restart redis
```

### ❌ Telegram bot не отвечает

```bash
# Логи бота
docker-compose logs bot --tail 50

# Проверить что токен установлен
grep TELEGRAM_TOKEN .env

# Проверить connectivity
docker exec vpn_bot ping -c 3 api.telegram.org

# Перезагрузить бота
docker-compose restart bot
```

---

## 🔄 Обновления

### Обновить код

```bash
git pull
docker-compose up -d --build
```

### Обновить Docker образы

```bash
docker-compose pull
docker-compose up -d
```

---

## 🎯 Структура файлов

```
/opt/vpnplatform/
├── .env                          # Переменные окружения (используется везде)
├── .env.example                  # Шаблон .env
├── docker-compose.yml            # Один универсальный конфиг
├── Caddyfile                     # HTTPS конфигурация
│
├── cmd/                          # Go приложения
│   ├── api/main.go              # Backend REST API
│   ├── bot/main.go              # Telegram бот
│   └── worker/main.go           # Background jobs
│
├── frontend/                     # React приложение
│   ├── Dockerfile
│   ├── nginx.conf
│   └── src/
│
├── deployments/                  # Docker образы
│   ├── Dockerfile.backend
│   ├── Dockerfile.bot
│   └── Dockerfile.worker
│
├── pgbackups/                    # Автоматические backup'ы БД
│   └── vpnplatform_YYYYMMDD.sql.gz
│
└── DEPLOY.md                     # Эта инструкция
```

---

## ✨ Рекомендации

1. **Регулярно обновляй код**
   ```bash
   cd /opt/vpnplatform && git pull && docker-compose up -d --build
   ```

2. **Мониторь логи**
   ```bash
   docker-compose logs -f backend
   ```

3. **Делай бэкапы**
   ```bash
   docker exec vpn_postgres pg_dump -U vpn vpnplatform | gzip > backup-$(date +%Y%m%d).sql.gz
   scp -r user@server:/opt/vpnplatform/pgbackups ~/backups/
   ```

4. **Проверяй здоровье**
   ```bash
   docker-compose ps
   docker stats --no-stream
   ```

---

**✅ Готово! Платформа работает на https://mellowpn.space 🚀**
