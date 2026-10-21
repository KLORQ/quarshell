<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Credential-провайдеры

Credential-провайдеры привязывают именованные секреты и egress endpoints к
sandbox. Это не compute backend ([Docker](../providers/docker/index.md), [Podman](../providers/podman/index.md)).

## Store один раз

Секреты читаются из **env текущего процесса**, не из sticky `export` и не из
значений в argv.

```bash
GITHUB_TOKEN=… cautem provider create --name gh --type github --credential GITHUB_TOKEN
CURSOR_API_KEY=… cautem provider create --name cursor --type cursor --credential CURSOR_API_KEY

# если ключи уже в env этого процесса:
# cautem provider create --name gh --type github --from-existing

cautem provider list
cautem provider get gh
```

`list` / `get` никогда не печатают значения. Ciphertext — в encrypted store
gateway (`secrets.enc.json`).

## Attach на create

```bash
cautem sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy /path/to/policy.yaml \
  --provider gh \
  --provider cursor \
  --memory 2g
```

Composition подмешивает endpoints, binaries и `credential_keys` профиля в
effective policy. В guest — placeholders вида
`cautem:resolve:env:GITHUB_TOKEN`. Proxy делает rewrite на egress.

## Поведение профилей

| `--type` | Типичные ключи | Guest |
|----------|----------------|-------|
| `github` | `GITHUB_TOKEN` / `GH_TOKEN` | Placeholder + rewrite |
| `cursor` | `CURSOR_API_KEY` (опц.) | **Без** ключа (`inject_env: false`) → `agent login` |
| `nvidia` | `NVIDIA_API_KEY` | Placeholder |
| `claude-code` | `ANTHROPIC_API_KEY` | Placeholder |

```bash
cautem provider profile list
cautem provider profile show github
```

## Ротация

```bash
GITHUB_TOKEN=ghp_new… cautem provider refresh gh
# или: GITHUB_TOKEN=… cautem provider update gh --from-existing
```

## Не делайте

```bash
cautem sandbox create … --env GITHUB_TOKEN=ghp_…     # сырой секрет в guest
cautem sandbox create … --env CURSOR_API_KEY=cautem:resolve:…  # ломает Cursor Agent
```

## KEK

| Элемент | Деталь |
|---------|--------|
| Env | `CAUTEM_SECRETS_KEK` (passphrase, base64 или hex ≥16 байт) |
| File fallback | `secrets.kek` в data dir gateway (mode 0600) |
| Doctor | Предупреждает, если KEK не закреплён в env |

Закрепите KEK для compose и долгоживущих gateway, иначе recreate может
осиротить ciphertext.
