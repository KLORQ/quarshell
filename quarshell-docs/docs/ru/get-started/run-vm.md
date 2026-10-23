<!--
SPDX-FileCopyrightText: Copyright (c) 2026 the Quarshell authors
SPDX-License-Identifier: Apache-2.0
-->

# Локальная VM-песочница

Инструкция собирает Go CLI, Gateway и Linux runtime из workspace, затем
запускает sandbox в аппаратно виртуализированной Linux VM. Поддержка VM
экспериментальная, публичный API остаётся в серии `0.x`.

## Требования

- Версии Go и Task из текущего workspace, Git, Python 3 и `curl`.
- **macOS:** Apple silicon (arm64), Hypervisor.framework, Homebrew и пакеты
  `libkrun`, `libkrunfw`, `e2fsprogs` (`brew install libkrun libkrunfw e2fsprogs`).
- **Linux:** amd64 или arm64, включённый KVM и права чтения/записи на
  `/dev/kvm`, совместимый runtime bundle `libkrun` и `e2fsprogs` (`mke2fs`).
  Запускайте Gateway от обычного пользователя, не через `sudo`.
- Доступ в сеть для загрузки закреплённой версии `umoci` и выбранного OCI-образа.

## Сборка и настройка

Получите workspace с закреплёнными независимыми модулями и соберите runtime
helpers и хостовые бинарные файлы:

```sh
git clone https://github.com/KLORQ/quarshell-workspace.git
cd quarshell-workspace
tools/checkout-modules.sh
task workspace:sync

export VM_ASSETS="$PWD/quarshell-driver/dist/vm-local"
mkdir -p "$VM_ASSETS/libkrun"
QUARSHELL_VM_BUILD_DIR="$VM_ASSETS" quarshell-driver/scripts/build-vm-runtime.sh
go build -C quarshell-cli -o "$VM_ASSETS/quarshell" ./cmd/quarshell
go build -C quarshell-gateway -o "$VM_ASSETS/quarshell-gateway" ./cmd/quarshell-gateway
export PATH="$VM_ASSETS:$PATH"
```

На macOS свяжите библиотеки Homebrew и закрепите хеш локально собранного
worker с ad-hoc подписью. Этот вариант предназначен для Apple silicon:

```sh
ln -sf "$(brew --prefix libkrun)/lib/libkrun.dylib" "$VM_ASSETS/libkrun/libkrun.dylib"
ln -sf "$(brew --prefix libkrunfw)/lib/libkrunfw.5.dylib" "$VM_ASSETS/libkrun/libkrunfw.5.dylib"
export QUARSHELL_VM_WORKER_SHA256="$(shasum -a 256 "$VM_ASSETS/quarshell-vm-worker" | awk '{print $1}')"
export OPENSHELL_VM_RUNTIME_DIR="$VM_ASSETS/libkrun"
```

На Linux укажите подготовленный оператором bundle с файлами `libkrun.so` и
`libkrunfw.so.5`:

```sh
export OPENSHELL_VM_RUNTIME_DIR=/path/to/libkrun-bundle
```

При настройке окружения Gateway ниже оставьте это значение без изменений.

Настройте локальный Gateway на собранный runtime. Переменная
`QUARSHELL_GATEWAY_SERVICE=0` запускает Gateway без системного менеджера служб,
чтобы он унаследовал окружение этого shell. Задайте переменные до первого
`gateway ensure`; уже работающий Gateway нужно остановить и запустить снова
после смены backend.

```sh
export QUARSHELL_GATEWAY_RUNTIME=vm
export QUARSHELL_GATEWAY_SERVICE=0
export QUARSHELL_VM_WORKER="$VM_ASSETS/quarshell-vm-worker"
export QUARSHELL_VM_GUEST_AGENT="$VM_ASSETS/quarshell-vm-guest-agent-linux-$(go env GOARCH)"
export QUARSHELL_RUNTIME_INIT_BINARY="$VM_ASSETS/quarshell-init-linux-$(go env GOARCH)"
export QUARSHELL_RUNTIME_SUPERVISOR_BINARY="$VM_ASSETS/quarshell-driver-linux-$(go env GOARCH)"
export QUARSHELL_RUNTIME_SSHD_BINARY="$VM_ASSETS/quarshell-sshd-linux-$(go env GOARCH)"
export QUARSHELL_RUNTIME_PROXY_BINARY="$VM_ASSETS/quarshell"
export QUARSHELL_RUNTIME_POLICY_PATH="$PWD/quarshell-cli/policies/default.yaml"
```

На Linux `go env GOARCH` должен совпадать с архитектурой хоста (`amd64` или
`arm64`). На macOS он выбирает архитектуру Linux-гостя (`arm64`). При запуске
Gateway создаст локальные TLS credentials.

## Запуск sandbox

Запустите Gateway с VM backend и выберите опубликованный базовый образ:

```sh
quarshell gateway ensure
quarshell status
export QUARSHELL_USE_GHCR=1
```

Перейдите в каталог проекта, который нужно скопировать в гостевую VM:

```sh
quarshell sandbox create --name vm-demo --from base --upload "$PWD"
quarshell sandbox exec vm-demo -- uname -a
quarshell sandbox connect vm-demo
quarshell sandbox delete vm-demo
```

`--upload` копирует проект в VM, но не монтирует каталог хоста. На обеих
платформах workload работает в Linux VM. Назначение модулей описано в разделе
[Модули проекта](../concepts/modules.md).

## Устранение проблем

- **Ошибка доверия worker на macOS:** пересоберите worker, обновите
  `QUARSHELL_VM_WORKER_SHA256` и перезапустите Gateway. Для распространяемых
  сборок используйте подпись с Team ID.
- **Ошибка проверки KVM на Linux:** проверьте
  `test -r /dev/kvm && test -w /dev/kvm` от того же пользователя, что запускает Gateway.
- **Ошибка библиотек runtime:** проверьте, что в bundle лежат два платформенных
  файла `libkrun`, указанные выше.
- **Gateway сообщает о Docker:** остановите прежний Gateway, задайте VM-переменные
  в том же shell и повторите `quarshell gateway ensure`.
