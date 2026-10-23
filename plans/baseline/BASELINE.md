# Baseline: OpenShell v0.1.4

Зафиксировано: 2026-10-21.

| Параметр | Значение |
| --- | --- |
| Upstream version | `v0.1.4` |
| Upstream commit | `5a7728c304b5528e954e9ae900cf21f84e1a12b1` |
| Upstream checkout | `tools/upstream/openshell-v0.1.4` |
| Product name | Quarshell |
| Go toolchain | `go1.27.2` |
| Host ревью | `darwin/arm64`, macOS 27.0 |
| Review artifact | `plans/openshell-go-2026-10-21` |
| Source fingerprints | `plans/openshell-go-2026-10-21/source-sha256.txt` (1377 записей) |

Следующий upstream-релиз не подменяет baseline внутри текущих работ. Его приём
оформляется отдельным пересмотром inventory.

## Замороженные документы

| Документ | SHA-256 |
| --- | --- |
| `PLAN.md` | `ea8499dd278b07db7ae8311492df82b97cb0f87937ae9d10199142b7c89e1fee` |
| `REVIEW.md` | `5c6e6144e19f42cf6d014757e3470bf63133746b2030bdeaa7b63c6c75d0999a` |
| `rpc-inventory.csv` | `343eea8ee6be8c68bec7b3627c892e268736713e91a46193e6b47fa3301d82a9` |
| `source-sha256.txt` | `2fb4d19b24e81ce9f1d3a1d038a7cde7b64deabc62fb58119f8ea3dd0b5317dc` |

## Состояние рабочих деревьев на 2026-10-21

Корень workspace не является Git-репозиторием: каждый модуль версионируется
отдельно. Число изменённых entries — `git status --porcelain`, включая
неотслеживаемые файлы.

| Модуль | HEAD | Изменённых entries |
| --- | --- | --- |
| `quarshell-cli` | `1ffe76c` | 159 |
| `quarshell-core` | `81a360e` | 34 |
| `quarshell-display` | `83527b2` | 11 |
| `quarshell-docs` | `1604171` | 54 |
| `quarshell-driver` | `e2055b7` | 157 |
| `quarshell-gateway` | `c9590ac` | 250 |
| `quarshell-org-profile` | `113eac0` | 1 |
| `quarshell-proto` | `f116012` | 32 |
| `quarshell-policy` | `4f0872d` | 12 |
| `quarshell-proxy` | `3eb55a7` | 51 |
| `quarshell-python` | `cfb8738` | 31 |
| `quarshell-runtime` | `86c56b3` | 83 |
| `quarshell-sdk` | `77d8fd7` | 65 |
| `quarshell-slogx` | `d8bc237` | 20 |
| `quarshell-ui` | `5aff014` | 19 |

Незакоммиченные изменения — часть фактического baseline ревью. Они не считаются
принятыми в новую реализацию автоматически: новый код пишется с нуля по
функциональному контракту.
