<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Создание sandbox (Gateway на Docker)

## Минимальный create

```bash
cautem sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy quarshell-cli/policies/default.yaml \
  --memory 2g
```

С образом агента и credential-провайдерами:

```bash
cautem sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy /path/to/policy.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g
```

## Жизненный цикл

```bash
cautem sandbox list
cautem sandbox status demo
cautem sandbox exec demo -- uname -a
cautem sandbox connect demo          # подключение к sandbox через Gateway
cautem sandbox stop demo
cautem sandbox start demo
cautem sandbox delete demo
```

Команды жизненного цикла работают с sandbox выбранного Gateway. Docker должен
быть настроен на хосте Gateway; CLI не создаёт локальные Docker-контейнеры.

## Что делает create

1. Резолвит образ или общий workload template.
2. Передаёт policy, providers, ресурсы, labels, environment и команду через
   нативный Gateway RPC `CreateSandbox`.
3. Загружает указанные файлы нативным exec stream sandbox.
4. Запускает foreground-команду через native interactive exec или оставляет
   detached-команду запущенной в Gateway.

## Шаблоны

Фиксируйте sizing в template (как в OpenShell):

```bash
cautem sandbox template create \
  --name desk \
  --from cursor \
  --memory 2g \
  --cpu 2 \

cautem sandbox create --template desk --name worker --upload "$PWD"
```

Флаги перекрывают поля template. Soft defaults из config/env заполняют только
пустые поля — см. [Ресурсы](./resources.md).

По умолчанию шаблоны хранятся локально в профиле CLI. Флаг `--gateway`
управляет шаблоном в workspace выбранного Gateway:

```bash
cautem sandbox template create shared-agent --gateway \
  --image ghcr.io/example/agent:stable --cpu 500m --memory 2Gi \
  --gpu 1 --env MODE=fast --label team=runtime
cautem sandbox template list --gateway
cautem sandbox template get shared-agent --gateway
cautem sandbox template delete shared-agent --gateway
```

Общие шаблоны хранятся в Gateway. `sandbox create --template` использует
выбранный workspace Gateway.
