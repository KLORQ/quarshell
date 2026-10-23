<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Запуск Gateway

Для работы Go Gateway нужен приватный `QUARSHELL_TOKEN`. HTTP-запросы и gRPC
вызовы передают его как Bearer credential; исключения — HTTP liveness и gRPC
`Health`. Храните токен в переменной окружения или локальном игнорируемом
`.env`. Общий токен представляет статического администратора `gateway-client`,
но не отдельного пользователя или sandbox.

### Docker Compose

Запустите команды из корня репозитория. Сначала создайте локальный токен:

```sh
export QUARSHELL_TOKEN="$(openssl rand -hex 32)"
docker compose -f quarshell-gateway/compose/docker-compose.yml up --build -d
curl --fail https://127.0.0.1:17670/v1/healthz
```

Compose хранит состояние в volume `quarshell-gateway-data`. HTTP доступен на
`127.0.0.1:17670`, gRPC — на `127.0.0.1:7445`.
Для gRPC не настроен TLS listener. Оставьте порт `7445` на loopback или в
доверенной приватной сети; не публикуйте его для недоверенных клиентов.

### Локальный бинарный файл

Запускайте из корня репозитория:

```sh
export QUARSHELL_TOKEN="$(openssl rand -hex 32)"
go -C quarshell-gateway run ./cmd/quarshell-gateway \
  --listen 127.0.0.1:17670 \
  --grpc-listen 127.0.0.1:7444 \
  --runtime docker
```

Без `--data` Gateway хранит состояние в памяти. Укажите `--data ./state.db`,
чтобы использовать SQLite. Endpoint `/v1/healthz` возвращает только
`{"status":"ok"}`.

Профили UI и Dex сохранены из базового проекта, но не подключены к текущему
Gateway с token-аутентификацией. Текущий scope описан в
[README модуля Gateway](https://github.com/KLORQ/quarshell/blob/master/quarshell-gateway/README.md)
и [списке оставшейся работы](https://github.com/KLORQ/sandbox.dev/blob/main/docs/ru/reference/remaining-migration-work.md).
