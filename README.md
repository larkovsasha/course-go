# TripGo

Реализация 2 лабораторной работы «Разработка микросервисов на Go»: архитектура, тесты и observability. API и бизнес-правила ЛР1 сохранены.

## Требования

- Go 1.27.1
- `tripgoctl`
- `golangci-lint` v2
- Docker для контейнерного запуска

## Локальный запуск

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`cluster start` выполняется единожды. Окружение включает PostgreSQL и observability.
`environment start` создаёт `.env` с адресами БД и OTel Collector. Дополните недостающие поля по [.env.example](.env.example), сохранив адреса окружения.
`Makefile` загружает `.env` для всех целей. Миграции применяются до запуска; доступность БД проверяется при старте.

## Переменные окружения

Примеры значений - в [.env.example](.env.example). Все переменные обязательны; длительности задаются в формате Go (`s`, `m`, `h`) и должны быть положительными.

| Переменная | Назначение |
|---|---|
| `HTTP_ADDR` | Адрес HTTP-сервера |
| `LOG_LEVEL` | Уровень JSON-логов: `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | Таймаут остановки компонентов |
| `DATABASE_URL` | Подключение к PostgreSQL из `.env` |
| `DATABASE_MAX_CONNS` | Максимальное число соединений пула |
| `DATABASE_MIN_CONNS` | Минимальное число соединений пула |
| `DATABASE_MAX_CONN_LIFETIME` | Максимальное время жизни соединения |
| `DATABASE_CONNECT_TIMEOUT` | Таймаут подключения и стартовой проверки БД |
| `DATABASE_QUERY_TIMEOUT` | Таймаут запросов к БД |
| `HTTP_READ_TIMEOUT` | Таймаут чтения запроса |
| `HTTP_READ_HEADER_TIMEOUT` | Таймаут чтения заголовков |
| `HTTP_WRITE_TIMEOUT` | Таймаут записи ответа |
| `HTTP_IDLE_TIMEOUT` | Таймаут неактивного соединения |
| `IDEMPOTENCY_TTL` | Срок действия ключа идемпотентности |
| `IDEMPOTENCY_CLEANUP_INTERVAL` | Период очистки просроченных ключей |
| `OTEL_SERVICE_NAME` | Имя сервиса |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Адрес OTel Collector из `.env` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | Протокол экспорта: `http/protobuf` |
| `OTEL_RESOURCE_ATTRIBUTES` | Атрибуты ресурса, включая namespace и окружение |
| `OTEL_TRACES_SAMPLER` | `parentbased_always_on`, `always_on` или `always_off` |

## HTTP API

| Метод и путь | Результат |
|---|---|
| `POST /api/v1/trips` | Создание поездки: `201` и `Location` |
| `GET /api/v1/trips/{tripId}` | Получение поездки: `200` |
| `POST /api/v1/trips/{tripId}/finish` | Завершение поездки: `200` |
| `GET /health` | `200`, без обращения к БД |
| `GET /ready` | Проверка БД: `200` или `503` |

Ошибки поездок возвращаются как `application/problem+json`: `invalid_request` (`400`), `trip_not_found` (`404`), `driver_busy`, `trip_completed`, `idempotency_conflict` (`409`), `internal_error` (`500`). Полный контракт - в [OpenAPI](contracts/openapi/trip-service.openapi.yaml).

Пример создания при `HTTP_ADDR=:8080`:

```bash
curl -i -X POST http://localhost:8080/api/v1/trips \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":59.9398,"longitude":30.3146},"end_point":{"latitude":59.9290,"longitude":30.3626},"price":1450}'
```

Для идемпотентности передайте `Idempotency-Key` со значением UUID: первый запрос вернёт `201`, повтор с тем же телом - `200` и ту же поездку, с другим телом - `409 idempotency_conflict`.

## Проверки и генерация

```bash
go build ./...
make lint
make test
make test-integration
make generate
```

`make test` запускает unit-тесты с `-race -count=2` и печатает покрытие. Интеграционные тесты требуют PostgreSQL из `.env` с применёнными миграциями и сами очищают свои данные.
`make generate` обновляет OpenAPI-код и моки gomock; версии генераторов закреплены в `go.mod`, отдельная установка не нужна.

## Архитектура

```text
HTTP → usecase → domain
         └─ интерфейсы ← repository / transaction / telemetry
container создаёт и связывает реализации; main управляет запуском.
```

- `domain/trip` - бизнес-правила и ошибки.
- `usecase/trip/*` - создание, чтение и завершение поездок.
- `http/*` - обработчики и валидация запросов.
- `repository/trip` - SQL-запросы.
- `infra/*` - подключение к БД, транзакции и телеметрия.
- `worker/*` - очистка ключей идемпотентности.

Интерфейсы объявлены в `contract.go` тех пакетов, которые их используют. Например, usecase описывает только нужные ему методы репозитория. Все зависимости создаются в `internal/container`, правила импортов проверяет `depguard`.

`http/domainerror` переводит доменные ошибки в HTTP-ответы. При внутренней ошибке клиент получает `500 internal_error` без подробностей.

## Принятые решения

- **Транзакции:** `Do` передаёт транзакцию через контекст. Поездка, история и ключ идемпотентности записываются атомарно; ошибка или panic вызывают rollback. Используется `READ COMMITTED`: операциям не нужен неизменный снимок данных.
- **Конкурентность:** частичный уникальный индекс запрещает две активные поездки одного водителя; условный `UPDATE` защищает от повторного завершения.
- **Идемпотентность:** UUID ключа, SHA-256 тела и ссылка на поездку хранятся в БД. Просроченные ключи очищает фоновый worker.

## Observability

Трейсы и метрики уходят по OTLP в Collector, далее - в Jaeger и Prometheus/Grafana. Ссылки и данные для входа печатает `tripgoctl connect`.

Middleware: `Recover → Tracing → Logging → Metrics → router`. JSON-логи в stdout содержат `trace_id` и `span_id`; HTTP span включает дочерние DB spans. В route используется шаблон без UUID. Счётчики поездок увеличиваются после успешной транзакции, число активных поездок измеряется по БД. Недоступность коллектора не останавливает API.

Результаты проверок и скриншоты - в [reports/review](reports/review/README.md).

## Docker

После подготовки окружения в Linux:

```bash
docker build -f deploy/Dockerfile -t tripgo-lab2:latest .
docker run --rm --network host --env-file .env tripgo-lab2:latest
```

Образ использует `distroless` и непривилегированного пользователя. Сеть хоста даёт доступ к компонентам из `.env`. Готовность сервиса проверяется через `/ready`.
