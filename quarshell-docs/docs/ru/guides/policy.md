<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Политика

Sandbox стартует в режиме default-deny. Базовый YAML плюс composition
credential-профилей дают effective allowlist, который enforcing egress proxy.

## Set и get

```bash
quarshell policy get demo --full
quarshell policy set demo --policy /path/to/policy.yaml --wait
```

`--wait` ждёт, пока sidecar перечитает bind-mounted effective YAML.

Сначала просмотрите результат incremental network policy update:

```bash
quarshell policy update demo --add-allow 'api.github.com:443,8443:GET:/repos/**' --rule-name github --binary /usr/bin/agent --dry-run
```

`--dry-run` читает текущую base policy, печатает объединённый YAML и не
отправляет update в Gateway.

Обновления только с удалением используют атомарный merge API Gateway:

```bash
quarshell policy update demo --remove-endpoint api.github.com:443
quarshell policy update demo --remove-rule github
quarshell policy update demo --add-endpoint api.example.com:443:read-only:rest --rule-name example-api --remove-rule old-api
```

`--add-endpoint`, L7 операции и удаления можно объединять атомарно. Для L7
обязательны `--rule-name` и полный binary scope правила через повторяемые
`--binary`, либо `--any-binary`, если правило разрешает все binaries. Формат
endpoint: `host:port[,port...]:METHOD:path_glob`; укажите все порты выбранного
endpoint. `--endpoint-path` выбирает endpoint с таким policy path (пустое
значение выбирает endpoint без path selector). `--dry-run` показывает
объединённый результат без обновления Gateway.
`--timeout SECONDS` задаёт deadline применения policy (по умолчанию 60 секунд).

## policy.local (guest)

Внутри sandbox агент может читать deny и предлагать узкие правила через
`policy.local` (MITM HTTPS к sidecar):

| Method | Path |
|--------|------|
| GET | `/v1/policy/current` |
| GET | `/v1/denials?last=N` |
| POST | `/v1/proposals` |
| GET | `/v1/proposals/{id}/wait` |

## Цикл approve у оператора

```bash
quarshell rule get --status pending
quarshell rule approve --chunk-id chk_…
# или: quarshell rule reject --chunk-id chk_… --reason "narrow to /docs"
```

Approve мержит правило в base policy sandbox, переписывает live policy file и
делает hot-reload sidecar. Предпочитайте минимальный `addRule`.

## Ключи схемы

Документы policy используют `filesystem_policy`, `landlock` и
`network_policies` (`quarshell-policy/corepolicy`). L7-правила могут задавать
`protocol: rest|graphql|mcp` с method/path/tool.

## Встроенная редактирующая middleware

Pinned middleware `openshell/regex` удаляет ключи API OpenAI из HTTP request
body и client-to-upstream WebSocket text:

```yaml
network_middlewares:
  redact-provider-keys:
    middleware: openshell/regex
    order: 10
    config:
      mode: redact
    on_error: fail_closed
    endpoints:
      include: ["api.example.com"]
```

`mode` можно опустить или задать `redact`; остальные поля и режимы отклоняются.
Фиксированное выражение `sk-[A-Za-z0-9_-]{16,}` заменяет совпадения на
`[REDACTED]`. Body и сообщения должны быть UTF-8 и не превышать 256 KiB.
Binary и server-to-client WebSocket сообщения не преобразуются; клиентские
binary сообщения, неверный UTF-8 и превышение лимита обрабатываются согласно
`on_error` (по умолчанию `fail_closed`). Finding содержит только тип,
severity, confidence и число совпадений. Ключ и исходный body в audit не
записываются.

## Связанное

- Skill: `/etc/quarshell/skills/policy-advisor`
