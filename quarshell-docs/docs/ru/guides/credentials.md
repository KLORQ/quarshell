<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Credential-провайдеры

Credential-провайдеры привязывают именованные секреты и egress endpoints к
sandbox. Это не compute backend ([Docker](../providers/docker/index.md), [Podman](../providers/podman/index.md)).

## Store один раз

Секреты читаются из **env текущего процесса**, не из sticky `export` и не из
значений в argv.

```bash
GITHUB_TOKEN=… quarshell provider create --name gh --type github --credential GITHUB_TOKEN
CURSOR_API_KEY=… quarshell provider create --name cursor --type cursor --credential CURSOR_API_KEY

# если ключи уже в env этого процесса:
# quarshell provider create --name gh --type github --from-existing

quarshell provider list
quarshell provider get gh
```

`list` / `get` никогда не печатают значения. Ciphertext — в encrypted store
gateway (`secrets.enc.json`).

## Attach на create

```bash
quarshell sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy /path/to/policy.yaml \
  --provider gh \
  --provider cursor \
  --memory 2g
```

Composition подмешивает endpoints, binaries и `credential_keys` профиля в
effective policy. В guest — placeholders вида
`quarshell:resolve:env:GITHUB_TOKEN`. Proxy делает rewrite на egress.

## Поведение профилей

| `--type` | Типичные ключи | Guest |
|----------|----------------|-------|
| `github` | `GITHUB_TOKEN` / `GH_TOKEN` | Placeholder + rewrite |
| `cursor` | `CURSOR_API_KEY` (опц.) | **Без** ключа (`inject_env: false`) → `agent login` |
| `nvidia` | `NVIDIA_API_KEY` | Placeholder |
| `claude-code` | `ANTHROPIC_API_KEY` | Placeholder |

```bash
quarshell provider profile list
quarshell provider profile show github
```

## Ротация

```bash
GITHUB_TOKEN=ghp_new… quarshell provider refresh gh
# или: GITHUB_TOKEN=… quarshell provider update gh --from-existing
```

## Не делайте

```bash
quarshell sandbox create … --env GITHUB_TOKEN=ghp_…     # сырой секрет в guest
quarshell sandbox create … --env CURSOR_API_KEY=quarshell:resolve:…  # ломает Cursor Agent
```

## KEK

| Элемент | Деталь |
|---------|--------|
| Env | `QUARSHELL_SECRETS_KEK` (passphrase, base64 или hex ≥16 байт) |
| File fallback | `secrets.kek` в data dir gateway (mode 0600) |
| Doctor | Предупреждает, если KEK не закреплён в env |

Закрепите KEK для compose и долгоживущих gateway, иначе recreate может
осиротить ciphertext.
