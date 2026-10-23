<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Создание sandbox (Gateway на Podman)

```bash
quarshell sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy quarshell-cli/policies/default.yaml \
  --memory 2g

quarshell sandbox exec demo -- uname -a
quarshell sandbox delete demo
```

С proxy и providers сначала выберите gateway:

```bash
quarshell gateway select local
quarshell sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy /path/to/policy.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g
```

Настройте Podman на хосте Gateway. `QUARSHELL_DRIVER=podman` в CLI не выбирает
compute driver удалённого Gateway. Сетевую policy применяет runtime Gateway.
