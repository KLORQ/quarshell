<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# CLI

```text
quarshell sandbox create|list|get|stop|start|delete|exec|connect|upload|download|…
quarshell cp <local-path> <sandbox>:/path | <sandbox>:/path <local-path>
quarshell sandbox template create|list|get|delete
quarshell sandbox provider …
quarshell provider create|list|get|refresh|update|delete|profile …
quarshell profile …
quarshell gateway add|select|ensure|info|login|…
quarshell policy get|set|…
quarshell rule get|approve|reject|…
quarshell workspace|forward|service|settings|whoami|logs|term
quarshell health|init|doctor|status|version|install|completions|ssh-proxy
```

`--gateway-endpoint URL` выбирает Gateway напрямую, без поиска имени в
зарегистрированных Gateway. Флаг можно указать до или после пути команды;
endpoint имеет приоритет над сохранённым выбором Gateway.
`--gateway-insecure` отключает проверку TLS-сертификата Gateway и предназначен
только для endpoint, сертификат которого локально нельзя проверить.
Повторяемый `-v` или `--verbose` включает debug logging.
`--color auto|always|never` управляет ANSI-цветами (`QUARSHELL_COLOR` задаёт
значение по умолчанию; в режиме `auto` непустой `NO_COLOR` отключает цвета,
а непустой `FORCE_COLOR` включает их, если `NO_COLOR` не задан).

`quarshell forward service --target-port PORT [--target-host HOST] [--local
[BIND:]PORT] [SANDBOX]` открывает локальный TCP listener к loopback-сервису в
sandbox. По умолчанию target host — `127.0.0.1`; `--local 0` просит ОС выбрать
локальный порт.

`quarshell provider list [--page-size N] [--page-token TOKEN]
[--label-selector EXPR] [--all-workspaces] [-o table|yaml|json]` возвращает
одну страницу Gateway. Передайте `next_page_token` как `--page-token`; для
all-workspaces нужен platform administrator. Значения provider credentials
остаются скрыты.
`quarshell provider list-profiles [--workspace NAME|--global] [--page-size N]
[--page-token TOKEN] [-o table|yaml|json]` постранично выводит выбранный
каталог профилей; те же параметры принимает `provider profile list`.

`quarshell term --theme auto|dark|light` выбирает акцентную палитру TUI;
`QUARSHELL_THEME` задаёт значение по умолчанию. Режим `auto` использует
`COLORFGBG`, если он задан, иначе выбирает тёмную тему.

## Create (compute)

| Флаг | Смысл |
|------|-------|
| `--name` | Имя sandbox |
| `--from` / `--image` | Alias или OCI reference |
| `--gateway-workspace` | Workspace OpenShell Gateway |
| `--policy` | YAML политики OpenShell SandboxPolicy |
| `--cpu` / `--memory` | Переносимые лимиты ресурсов |
| `--template` | Именованный workload template |
| `--provider` | Credential-провайдер (повтор) |
| `--expose PORT` | Зарегистрировать HTTP/WebSocket service вместе с Gateway create |
| `--expose-authorization-mode strip|bearer-passthrough` | Выбрать обработку Bearer authorization в service proxy |
| `--gpu` | NVIDIA CDI |

`quarshell sandbox create --help` показывает допустимые параметры создания.
Создание всегда использует выбранный Gateway и OpenShell `CreateSandbox`.
Локальная сборка, display, bind mount и Engine-специфичные create-флаги не
принимаются. У каждого sandbox leaf есть собственная справка:
`quarshell sandbox <leaf> --help`.

Укажите `--gateway-workspace NAME`, чтобы выбрать workspace. При создании из
образа доступны CPU, memory, количество GPU, environment, labels, provider
attachments, policy, restart policy и JSON конфигурации driver.
Общий шаблон содержит свои workload-параметры и не принимает inline overrides.
`--from` принимает локальный rootfs архив `.tar`, `.tar.gz` или `.tgz`, если
Gateway находится на этой машине, а compute driver поддерживает rootfs staging.
Foreground-команда запускается после provisioning через native interactive
exec; `--detach` оставляет её запущенной в Gateway. Для подключения IDE
используйте `quarshell sandbox connect NAME --editor vscode|cursor`.

`--expose PORT` сохраняет доступность sandbox supervisor. Gateway получает
exposure вместе с запросом создания и просит Gateway удалить новый sandbox,
если регистрация service завершилась ошибкой. Это компенсирующее удаление;
если cleanup не удался, Gateway сообщает об ошибке для reconciliation.
`--expose-authorization-mode` по умолчанию
использует `strip`; `bearer-passthrough` передаёт приложению один корректный
Bearer header.

Повторяйте `--forward PORT` при создании, чтобы после provisioning запустить
фоновый локальный TCP forward. `quarshell forward start [BIND:]PORT SANDBOX`
использует native OpenShell `ForwardTcp` relay к loopback-порту sandbox;
`--forward` использует одинаковый порт на локальной и sandbox стороне.

SSH helper из сгенерированных настроек `sandbox ssh-config` поддерживает
текущий Gateway, именованный Gateway и явный URL Gateway с токеном sandbox.
Точные варианты показывает `quarshell ssh-proxy --help`.

Host bind mount, локальная сборка образа, display и переключатели локального
runtime не входят в контракт OpenShell Gateway create. `--upload` передаёт
файлы через native OpenShell interactive execution RPC. Foreground-команда с
TTY запускается как canonical main process в create request, после чего CLI
подключается к сохранённому PTY. Команда без TTY запускается через native
interactive exec после создания. Detached-команда передаётся в create request.

Добавляйте `--gateway` к команде `sandbox template`, чтобы управлять шаблоном
в workspace выбранного Gateway. Gateway template create также принимает
повторяемые `--annotation KEY=VALUE`, `--ready-within DURATION` и
`--max-burst COUNT`; для локальных шаблонов эти параметры отклоняются, а не
игнорируются. Формат duration: `30s`, `5m` или `1h`.

## Подкоманды sandbox

| Команда | Формат |
|---------|--------|
| `sandbox list` | `--page-size N`, `--page-token TOKEN`, `--selector EXPR`, `-o table\|yaml\|json`, `--ids`, `--names`, `--all-workspaces` |
| `sandbox get` | `NAME [--policy-only]` |
| `sandbox start` / `sandbox stop` | Необязательное `NAME` (по умолчанию последний sandbox) |
| `sandbox delete` | `NAME...` либо отдельный `--all` |
| `sandbox upload` | `NAME LOCAL_PATH [SANDBOX_DEST] [--no-git-ignore]` |
| `sandbox download` | `NAME SANDBOX_PATH [LOCAL_DEST]` |
| `sandbox ssh-config` | `NAME [--install]` |
| `sandbox connect` | `[NAME] [--editor vscode\|cursor]` либо `[NAME] -- COMMAND...` |
| `sandbox provider list` | `[SANDBOX] [--page-size N] [--page-token TOKEN] [-o table\|yaml\|json]` |
| `sandbox provider attach` / `detach` | `SANDBOX PROVIDER` (выводит change ID) |
| `sandbox provider status` | `SANDBOX PROVIDER [--receipt ID] [--wait] [--timeout SECONDS] [-o table\|yaml\|json]` |

`sandbox list` возвращает одну страницу. Передайте `next_page_token` как
`--page-token`; table выводит token в stderr, JSON/YAML включают его в ответ.
Для `--all-workspaces` нужна роль platform administrator.
| `sandbox template create` | `NAME`; image/from/policy, CPU/memory/PIDs/GPU, env, labels, driver config и provider flags |
| `sandbox template list` | `--gateway [--page-size N] [--page-token TOKEN] [--label-selector EXPR] [-o table\|yaml\|json]` |
| `sandbox template get` / `delete` | `NAME [--gateway]` |

Список Gateway templates возвращает одну страницу. Передайте `next_page_token`
как `--page-token`; table выводит token в stderr, JSON/YAML включают его.

`--editor` нельзя сочетать с командой. `--no-git-ignore` доступен только для
upload; каталог по умолчанию для upload — `/workspace`, для download — текущий
каталог.

## Команды workspace

| Команда | Формат |
|---------|--------|
| `workspace create` | `--name NAME`, повторяемый `--label KEY=VALUE` |
| `workspace list` | `--page-size N`, `--page-token TOKEN`, `--label-selector KEY=VALUE[,KEY=VALUE...]`, `-o table\|yaml\|json` |
| `workspace member list` | `--workspace NAME`, `--page-size N`, `--page-token TOKEN`, `-o table\|yaml\|json` |
| `workspace get` | `NAME` |
| `workspace delete` | `NAME...` |

Команда списка возвращает одну страницу. Передайте её `next_page_token` в
`--page-token`, чтобы запросить следующую страницу. Условия label selector
объединяются через AND.
Список workspace members также возвращает одну страницу и принимает token
продолжения из предыдущего ответа.

## Settings

`quarshell settings get [SANDBOX] [--global] [--json]` читает эффективные
настройки последнего или указанного sandbox; `--global` читает настройки
Gateway и не принимает имя sandbox. `settings set [SANDBOX] --key KEY
--value VALUE` и `settings delete [SANDBOX] --key KEY` задают или удаляют
настройку sandbox. Для глобальной настройки укажите `--global --yes`. CLI
проверяет ключи и типы значений по реестру OpenShell.

## Service endpoints

Используйте `quarshell service expose SANDBOX PORT [NAME] [--authorization-mode strip|bearer-passthrough]`,
`service list [SANDBOX] [--page-size N] [--page-token TOKEN] [--all-workspaces] [-o table|yaml|json]` и `service get|delete SANDBOX [NAME]`. Список возвращает одну страницу: table выводит следующий token, JSON/YAML включают `next_page_token`. Для `--all-workspaces` нужна роль platform administrator; флаг переопределяет выбранный workspace. Текущий Gateway
поддерживает безопасный режим `strip`: перед proxy к приложению Authorization
headers удаляются. `bearer-passthrough` передаёт приложению один корректный
Bearer header без изменений по HTTP и WebSocket; этот режим требует соединения
с Gateway. По умолчанию используется `strip`. Таблица сервисов
показывает URL endpoint; JSON/YAML также содержат ID endpoint, sandbox и
workspace, domain и режим авторизации.

## Терминальный интерфейс

`quarshell term` обновляет список sandbox каждые две секунды. Выбирайте sandbox
клавишами `↑`/`↓` или `j`/`k`, нажмите `/` для фильтра по имени, статусу или
образу, `d` для просмотра подробностей. `enter` или `c` подключает к sandbox,
`e` открывает интерактивную оболочку, `s` останавливает работающий sandbox,
`S` запускает остановленный. Для удаления нажмите `x` и подтвердите действие.
`l` следит за логами выбранного sandbox, `a` — за логами всех sandbox, `L`
печатает логи выбранного sandbox и возвращается. `p` приостанавливает и
возобновляет отображение live-логов, пока ограниченный буфер продолжает их
собирать; `s` переключает source-фильтр (локально: `all`, `sandbox`, `proxy`;
Gateway: `all`, `gateway`, `sandbox`). `r`
В live-логах `↑`/`↓` или `j`/`k` выбирают строку, `Enter` открывает её детали,
`PgUp`/`PgDn` или `Ctrl-B`/`Ctrl-F` перемещают выбор на размер viewport, а `g`/`G` выбирают первую
или последнюю строку. `y` отправляет выбранную строку, а `Y` — видимые строки
логов в clipboard терминала; поддержка зависит от терминала. `y` копирует
текущую строку или выбранный визуальный диапазон. `r` обновляет
список, `q` закрывает интерфейс. `v` начинает или завершает цветом выделенный
визуальный диапазон; перемещайтесь `j`/`k`, затем `y` копирует выбранные строки. Нажмите
`Esc`, чтобы выйти из подробностей или просмотра потока логов.

## Soft defaults

Если флаги (и template) не задают sizing, config/env могут заполнить пробелы —
жёсткого default memory нет. См. [Ресурсы Docker](../providers/docker/resources.md).
