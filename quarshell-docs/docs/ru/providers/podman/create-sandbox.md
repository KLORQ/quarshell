<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Создание sandbox (Gateway на Podman)

```bash
cautem sandbox create \
  --name demo \
  --upload "$PWD" \
  --policy quarshell-cli/policies/default.yaml \
  --memory 2g

cautem sandbox exec demo -- uname -a
cautem sandbox delete demo
```

С proxy и providers сначала выберите gateway:

```bash
cautem gateway select local
cautem sandbox create \
  --name cursor \
  --from cursor \
  --upload "$PWD" \
  --policy /path/to/policy.yaml \
  --provider cursor \
  --provider gh \
  --memory 2g
```

Настройте Podman на хосте Gateway. `CAUTEM_DRIVER=podman` в CLI не выбирает
compute driver удалённого Gateway. Сетевую policy применяет runtime Gateway.
