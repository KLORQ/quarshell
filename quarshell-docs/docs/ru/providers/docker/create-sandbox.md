<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Создание sandbox (Gateway на Docker)

## Минимальный create

```bash
quarshell sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy quarshell-cli/policies/default.yaml \
  --memory 2g
```

С образом агента и credential-провайдерами:

```bash
quarshell sandbox create \
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
quarshell sandbox list
quarshell sandbox status demo
quarshell sandbox exec demo -- uname -a
quarshell sandbox connect demo          # подключение к sandbox через Gateway
quarshell sandbox stop demo
quarshell sandbox start demo
quarshell sandbox delete demo
```

Команды жизненного цикла работают с sandbox выбранного Gateway. Docker должен
быть настроен на хосте Gateway; CLI не создаёт локальные Docker-контейнеры.

## Что делает create

1. Сначала ищет template в профиле CLI; если его нет, использует общий
   workload template из выбранного workspace Gateway.
2. Передаёт policy, providers, ресурсы, labels, environment и команду через
   нативный Gateway RPC `CreateSandbox`.
3. Загружает указанные файлы нативным exec stream sandbox.
4. Запускает foreground-команду через native interactive exec или оставляет
   detached-команду запущенной в Gateway.

## Шаблоны

Фиксируйте sizing в template (как в OpenShell):

```bash
quarshell sandbox template create \
  --name desk \
  --from cursor \
  --memory 2g \
  --cpu 2 \

quarshell sandbox create --template desk --name worker --upload "$PWD"
```

Template задаёт workload defaults. При создании нельзя переопределять его
image, source, CPU, memory, GPU, environment и driver config. Policy, providers,
labels и forwards можно указать отдельно. См. [Ресурсы](./resources.md).

По умолчанию шаблоны хранятся локально в профиле CLI. Флаг `--gateway`
управляет шаблоном в workspace выбранного Gateway:

```bash
quarshell sandbox template create shared-agent --gateway \
  --image ghcr.io/example/agent:stable --cpu 500m --memory 2Gi \
  --gpu 1 --env MODE=fast --label team=runtime
quarshell sandbox template list --gateway
quarshell sandbox template get shared-agent --gateway
quarshell sandbox template delete shared-agent --gateway
```

Общие шаблоны хранятся в Gateway. Если локального template с таким именем нет,
`sandbox create --template` ищет его в выбранном workspace Gateway.
