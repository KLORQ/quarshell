---
status: soon
---

<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# MicroVM

<div class="ws-soon" markdown>

![В разработке](../../assets/images/under-construction.svg){ width="420" }

**Экспериментальный режим.** Локальный драйвер работает, но Gateway пока его не включает.

</div>

## Что уже работает

Драйвер запускает отдельную libkrun VM для каждой песочницы. Linux rootfs можно
импортировать из OCI registry reference или staged tar-архива; драйвер создаёт
неизменяемый ext4 base image и writable overlay для каждой песочницы. Для
registry используются Docker/Podman credentials процесса драйвера. Выбирается
платформа `linux/<архитектура хоста>`; перед кешированием проверяются manifest
и layer digests.

Также есть аутентифицированный guest control channel с versioned v1 JSON и
length-prefixed frames; host и guest отклоняют пропущенные и неизвестные версии.
`start-workload` возвращает успех только после того, как guest agent прочитал
закрытый versioned readiness marker, созданный supervisor после запуска workload.
Путь к marker удаляется из environment дочернего процесса. Также доступны
перезапуск процесса, exec, logs, file copy, policy proxy и SSH relay. Host
вращает per-sandbox token при успешном Start и recovery; pending rotation
хранится в закрытом каталоге sandbox и повторяется с тем же rotation ID после
потери ответа.
Exec поддерживает одноразовые команды и full-duplex PTY с stdin, resize окна и
terminal-generated signals. Gateway вызывает этот VM path через native Backend
extension `ExecTerminal`. VM workspace теперь задаётся как persistent guest overlay:
host-каталог в VM не передаётся. Создание sandbox остаётся fail-closed до реализации
host supervisor/isolation contract и прохождения target-host gates.

Образы
хранятся в кеше внутри каталога состояния VM. Квота по умолчанию — 64 GiB;
неиспользуемые образы удаляются, когда требуется место.

## Готовность к production

Linux worker запускается в собственных user и mount namespaces и делает
распространение mounts приватным до создания broker и VM. Запуск завершается
ошибкой, если namespaces недоступны или Gateway запущен от host root. Namespace
root worker отображается на account Gateway. macOS запускает отдельный worker
process. Host workspace mounts не используются: persistent ext4 overlay внутри
VM служит рабочим пространством, как в OpenShell. Gateway request builder уже
выбирает этот режим, но само создание VM пока отключено.

Gateway отклоняет создание VM до реализации OpenShell-style host supervisor и
аутентифицированной host/guest isolation contract, а также прохождения Gateway
acceptance для Linux KVM и macOS ARM64.

Поля OCI image `Entrypoint`, `Cmd`, `Env`, `User` и `WorkingDir` применяются к
in-memory VM workload spec. При Gateway recovery image config загружается по
закреплённому platform manifest digest. Environment и user не сохраняются в
durable state, reserved identity variables фильтруются, root identity отклоняется.
Unit tests и macOS ARM64 live acceptance проверяют OCI config, credential-helper auth
приватного registry, восстановление по digest, одноразовый TTY exec и наследование
image env/workdir. Та же live проверка подтверждает PTY resize, stdin и Ctrl-C.
Gateway→VM и Linux KVM acceptance, host supervisor/isolation, GPU passthrough и
PID limits остаются открытыми. Эти пробелы перечислены в
[матрице runtime acceptance](../reference/runtime-acceptance.md).

## Настройка хоста

Соберите worker, статический Linux guest agent, umoci и runtime helpers командой
`quarshell-driver/scripts/build-vm-runtime.sh`. Укажите `CAUTEM_VM_WORKER`,
`CAUTEM_VM_GUEST_AGENT` и OpenShell-переменную `OPENSHELL_VM_RUNTIME_DIR`.
VM state по умолчанию хранится в `$XDG_STATE_HOME/openshell/vm-driver` (или
`$HOME/.local/state/openshell/vm-driver`); для override задайте
`OPENSHELL_VM_DRIVER_STATE_DIR`. Rootfs tar по умолчанию staging-ится в
`<state_dir>/rootfs-tar-staging`; задайте `rootfs_tar_staging_dir` или
`OPENSHELL_VM_ROOTFS_TAR_STAGING_DIR`, чтобы выбрать другой каталог.
Если `OPENSHELL_VM_RUNTIME_DIR` не задан, Go driver ищет runtime bundle в
`$XDG_DATA_HOME/openshell/vm-runtime` (или
`$HOME/.local/share/openshell/vm-runtime`). Bundle по-прежнему устанавливает
оператор: в отличие от upstream embedded build resources, этот Go build пока
не встраивает и не извлекает libkrun.
Сборка берёт umoci v0.6.0, проверяет SHA-256 по
`quarshell-driver/scripts/vm-runtime-artifacts.lock`. Для новой версии сначала
добавьте проверенный digest в lockfile. `CAUTEM_VM_UMOCI` задаёт готовый
бинарник и требует `CAUTEM_VM_UMOCI_SHA256`; эти значения проверяются до
публикации в runtime каталоге. После полной сборки создаётся
`vm-runtime-manifest.json` с хешами всех семи артефактов; проверка manifest
отклоняет отсутствующие, изменённые и лишние файлы. Linux требует чтение и запись в `/dev/kvm`;
macOS поддерживается на Apple Silicon с Hypervisor.framework.
Gateway preflight требует подписать macOS worker доверенной Apple Developer
командой: задайте `CAUTEM_VM_CODESIGN_IDENTITY` при сборке и совпадающий
`CAUTEM_VM_WORKER_TEAM_ID` при запуске Gateway. Проверка выполняет
`codesign --verify --strict` и сравнивает Team ID; ad-hoc подпись отклоняется.
Для сборки rootfs нужен e2fsprogs.

В двухдисковом Create host stages tar или OCI layout на временный ext4 disk,
запускает image-prep worker на pinned bootstrap и проверяет `/image-rootfs`
перед публикацией immutable workload cache entry. Это проверено live на macOS
ARM64; Linux KVM lanes и полный отказной/security набор остаются release gates.

`CAUTEM_VM_IMAGE_CACHE_MAX_BYTES` меняет квоту кеша с 64 GiB по умолчанию.
Credentials registry берутся из Docker или Podman credential files,
доступных процессу драйвера. Загруженный rootfs tar ограничен 10 GiB в сжатом
виде и 32 GiB после распаковки.

Инструкции live acceptance драйвера находятся в разделе MicroVM в
[README драйвера](https://github.com/KLORQ/quarshell-driver#microvm-status).
Gateway и Linux KVM acceptance остаются обязательными gates релиза.


Полный список parity задач, production этапов, тестовых матриц и release
criteria приведён в [плане MicroVM до релиза](microvm-release-plan.md).
