# ADR-0001: Compute и Isolation — разные контракты

- Статус: Accepted
- Дата: 2026-10-21

## Контекст

Текущие driver/runtime смешивают выделение ресурсов и ограничения внутри
выделенной среды. Это мешает независимо проверять backends и security boundary.

## Решение

`Compute` отвечает за выделение container/pod/VM/session, сеть, storage, GPU,
lifecycle и lookup по устойчивому ID. `Isolation` отвечает за допуск workload к
filesystem/network/process operations внутри среды.

`PreparedLaunch` — opaque handle, привязанный к sandbox/generation. `Probe` не
обещает будущую готовность; `Prepare` перепроверяет ограничения перед запуском.
`Ensure` ищет ресурсы по stable labels/ID после ambiguous timeout.

Optional capabilities (exec, PTY, file transfer, service forward, policy update,
rootfs staging, native events, GPU) не расширяют required interface всех
backends. Registry проверяет capability, implementation, protocol version и host
profile.

## Последствия

- Docker/Podman/Kubernetes/VM/MXC реализуют общий Compute contract suite.
- Linux launcher/broker реализует Isolation независимо от compute backend.
- Нельзя добавить метод в required interface только потому, что он есть у одного
  backend.
- Ошибка capability обнаруживается до side effects.
