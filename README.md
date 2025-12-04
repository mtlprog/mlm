# mlm

MLM (Montelibero Multi-Level Marketing) — CLI для распределения наград EURMTL участникам сети Stellar на основе MTLAP токенов и рекомендаций.

## Установка

```bash
go install github.com/mtlprog/mlm/cmd/mlmc@latest
```

## Использование

### Команды

#### `mlmc report dry`

Генерирует предварительный отчёт без сохранения в базу данных. Полезно для проверки расчётов перед фактическим распределением.

```bash
mlmc report dry
mlmc --notify-tg report dry  # с уведомлением в Telegram
```

#### `mlmc report create`

Генерирует отчёт и сохраняет его в базу данных. Транзакция не отправляется.

```bash
mlmc report create
mlmc --notify-tg report create  # с уведомлением в Telegram
```

#### `mlmc distribute`

Отправляет транзакцию в сеть Stellar. Если в базе есть неотправленный отчёт за последние 24 часа — использует его. Иначе создаёт новый отчёт и отправляет транзакцию.

```bash
mlmc distribute
mlmc --notify-tg distribute  # с уведомлением в Telegram
```

### Флаги

- `--notify-tg` — отправить уведомление в Telegram после выполнения команды

## Конфигурация

Переменные окружения (можно указать в `.env`):

| Переменная | Описание |
|------------|----------|
| `POSTGRES_DSN` | Строка подключения к PostgreSQL |
| `TELEGRAM_TOKEN` | Токен Telegram бота |
| `STELLAR_ADDRESS` | Адрес кошелька для распределения |
| `STELLAR_SEED` | Секретный ключ для подписи транзакций |
| `REPORT_TO_CHAT_ID` | ID чата для отправки отчётов |
| `REPORT_TO_MESSAGE_THREAD_ID` | ID треда в чате (опционально) |

## Разработка

```bash
make build    # Генерация sqlc кода
make test     # Запуск тестов
make run      # Запуск mlmc
```

### Миграции

```bash
make migrate-up                   # Применить миграции
make migrate-status               # Статус миграций
make migrate-generate name=foo    # Создать новую миграцию
```
