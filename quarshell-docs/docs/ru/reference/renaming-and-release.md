<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Продуктовый и релизный контракт Quarshell

Имя продукта — Quarshell. Владелец GitHub остаётся `quarshell`; поддерживаемые
репозитории продукта и Go-модули используют имена `quarshell-*`. OpenShell —
цель совместимости, а не псевдоним продукта Quarshell.

## Политика совместимости

До этого переключения Quarshell не обещает обратную совместимость с прежними
релизами Quarshell или Quarshell. В бете будет одна актуальная версия CLI,
API/schema, пакетов, расположения конфигурации, пространства имён переменных
среды и идентичности runtime-ресурсов. Старые module paths, binary aliases,
имена конфигурации, API routes, форматы БД и adoption старых ресурсов не
поддерживаются. Мы не будем выпускать dual readers, fallback paths,
compatibility adapters или migration tool Quarshell-to-Quarshell. Для установки
новой версии потребуется чистая установка; до замены старой сборки пользователь
самостоятельно сохраняет нужные данные вне Quarshell. Новый релиз не должен
молча удалять не принадлежащие ему старые ресурсы хоста.

До первой production beta допустимы breaking changes. После неё поддерживается
только явно описанный контракт Quarshell beta; совместимость с предыдущими
pre-beta сборками не подразумевается.

## Цель совместимости с OpenShell

Цель — последний стабильный релиз OpenShell, закреплённый tag и полным commit
SHA. На 2026-10-19 страница upstream Releases помечает v0.1.4 последним
стабильным релизом; движущиеся `main`/`dev` сборки не входят в цель. Активная
база — [OpenShell v0.1.4](https://github.com/NVIDIA/OpenShell/releases/tag/v0.1.4),
commit `5a7728c304b5528e954e9ae900cf21f84e1a12b1`.

«Полная совместимость» означает поведенческий parity для каждой заявленной
поверхности OpenShell: публичные RPC и семантика streams, CLI-команды и опции,
конфигурация/defaults/errors, lifecycle providers и credentials, границы
policy/filesystem/network/process, поведение Docker/Podman/MicroVM, семантика
SDK, TUI flows и устанавливаемые артефакты. Совпадения protobuf descriptors или
имён команд недостаточно. Для каждой возможности upstream нужны реализация
Quarshell и проходящий black-box/conformance test. Отсутствующая или частичная
обязательная возможность блокирует бету; её нельзя скрывать adapter-ом или
объявлять поддерживаемой.

После нового стабильного релиза OpenShell pin и полный inventory обновляются и
квалифицируются отдельным изменением релиза. Нельзя отслеживать неприкреплённую
ветку или незаметно смешивать контракты разных версий upstream.

## Канонические имена и переключение

- Go imports и module dependencies используют только
  `github.com/KLORQ/quarshell-*`.
- Команды продукта, package metadata, config paths, переменные среды, labels,
  images и новые runtime-ресурсы используют только имена Quarshell.
- Имена OpenShell wire остаются неизменными там, где это требуется для точного
  соответствия upstream-протоколу.
- Из поддерживаемого пути удаляются старые Quarshell aliases, импорты старых путей,
  двойная обработка имён переменных, legacy readers конфигурации/БД и adoption
  старых ресурсов. Идентификаторы протокола, принадлежащие upstream, не
  переименовываются.
- Release candidate собирается по одному неизменяемому source lock из 15
  репозиториев. В published consumer graph запрещены локальные `replace`,
  соседние checkout, dirty worktrees и lock со статусом `baseline-only`.

## Статус релиза

Текущий source lock всё ещё `baseline-only`, а в матрице совместимости открыты
поведенческие и host-security gates. Этот контракт не утверждает, что полный
parity уже достигнут. Публикация беты заблокирована до прохождения полного
conformance inventory, backend/security acceptance, чистой сборки published
consumer, подписи артефактов и тестов устанавливаемого релиза.
