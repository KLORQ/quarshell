---
status: soon
---

<!--
SPDX-FileCopyrightText: Copyright (c) 2026 cautem
SPDX-License-Identifier: Apache-2.0
-->

# Kubernetes

<div class="ws-soon" markdown>

![В разработке](../../assets/images/under-construction.svg){ width="420" }

**Скоро.** Этот провайдер пока строится.

</div>

## Что планируется

- Песочницы запускаются как pod'ы, выбор — `CAUTEM_DRIVER=kubernetes`.
- Gateway поставляется Helm-чартом: постоянное хранилище, ServiceAccount,
  NetworkPolicy, Ingress и опционально OIDC.
- Те же политики, прокси и credential-провайдеры, что и на Docker.

Сейчас драйвер `kubernetes` — заглушка. Пока используйте
[Docker](docker/index.md) или [Podman](podman/index.md).
