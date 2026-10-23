<!--
SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
SPDX-License-Identifier: Apache-2.0
-->

# Профили провайдеров

Профили объединяют имена credentials, endpoint policy, binaries и правила
обнаружения в одном YAML-файле. quarshell импортирует профили OpenShell в
каталог gateway и использует их при создании provider instance и сборке
политики sandbox.

## Проверьте профиль

Выбирайте профиль, чьи hosts и пути binaries соответствуют образу, используемому
sandbox. Upstream-профили — примеры; перед импортом проверьте разрешения и пути.

```bash
quarshell profile lint --url https://raw.githubusercontent.com/NVIDIA/OpenShell/main/providers/codex.yaml
quarshell profile lint -f ./providers/codex.yaml
```

Lint проверяет OpenShell-схему и runtime-функции, нужные `provider create`.
Для неподдерживаемых refresh strategies и token grants он возвращает ошибку с
указанием поля. Lint не подключается к провайдеру и не проверяет credential.

## Импортируйте профиль

Можно импортировать один файл, HTTPS URL или все YAML/JSON-файлы каталога:

```bash
quarshell profile import --url https://raw.githubusercontent.com/NVIDIA/OpenShell/main/providers/openai.yaml
quarshell profile import -f ./providers/codex.yaml
quarshell profile import --from ./providers
quarshell profile list
quarshell profile describe codex -o yaml
```

Импорт создаёт запись в каталоге gateway и завершается ошибкой, если такой ID
уже существует. Для обновления сначала экспортируйте актуальный профиль,
отредактируйте его и передайте в `profile update`; gateway отклонит устаревшую
версию при конкурентном изменении.

По умолчанию `provider create` ищет профиль в каталоге выбранного workspace.
Флаг `provider create --global-profile` выбирает глобальный каталог; созданный
provider instance всё равно принадлежит выбранному workspace.

```bash
quarshell profile export codex -o yaml > codex.yaml
# отредактируйте codex.yaml
quarshell profile update -f codex.yaml
```

## Запустите Codex с API key

`--from-existing` сканирует только credentials из `discovery.credentials` и
собирает все непустые объявленные env aliases. Пустой список отключает discovery
credentials. Отсутствующие значения не вызывают ошибку discovery; команда
`provider create --from-existing` завершается ошибкой, если не найдены credentials или configuration.
Для `google-vertex-ai` discovery также читает пять Vertex-переменных project/region/base URL/publisher; явные `--config` values имеют приоритет. Поля профиля `source` и
`scope` задаёт сервер при экспорте; при импорте/обновлении они игнорируются.

Upstream Codex fixture описывает поля `CODEX_AUTH_*`. Для прямого
неинтерактивного запуска Codex CLI документированная переменная — `CODEX_API_KEY`;
для этого сценария используйте отдельный профиль под ваш образ. См. [официальную
справку по переменным окружения Codex](https://learn.chatgpt.com/docs/config-file/environment-variables).

Сохраните профиль как `codex-api.yaml`, затем проверьте и импортируйте его:

```bash
cat > codex-api.yaml <<'YAML'
id: codex-api
display_name: Codex API key
category: agent
discovery:
  credentials: [api_key]
credentials:
  - name: api_key
    env_vars: [CODEX_API_KEY]
    required: true
    auth_style: bearer
    header_name: authorization
endpoints:
  - host: api.openai.com
    port: 443
    protocol: rest
    access: read-write
    enforcement: enforce
binaries: [/usr/local/bin/codex, /usr/bin/codex]
YAML

quarshell profile lint -f codex-api.yaml
quarshell profile import -f codex-api.yaml
CODEX_API_KEY=… quarshell provider create --name codex --type codex-api --from-existing

quarshell sandbox create \
  --name codex-work \
  --upload "$PWD" \
  --provider codex
```

Профиль задаёт имена переменных credential; provider instance хранит реальные
значения в зашифрованном хранилище gateway. CLI не выводит credentials. В
sandbox попадают placeholders; egress proxy раскрывает их только для связанных
с профилем endpoints. Список endpoints и пути binaries должны соответствовать
образу — профиль не добавляет отсутствующий executable.

Refresh стратегии OpenShell `oauth2_refresh_token` и
`oauth2_client_credentials` связаны с созданием provider. Материал обновления
хранится в зашифрованном хранилище gateway, поля ответа записываются в указанные
credential keys, а получение sandbox secrets обновляет токен перед истечением.
Первое обновление можно вызвать вручную:
`quarshell provider refresh rotate NAME --credential-key ACCESS_TOKEN`.

Upstream Codex OAuth fixture описывает credentials `CODEX_AUTH_*`, но quarshell
не получает и не обновляет ChatGPT login и пока не реализует Codex workload
identity federation. Token grants, AWS STS role assumption, Google
service-account JWT и SigV4 signing также не поддерживаются runtime. Такие
профили можно импортировать и экспортировать, но `profile lint` и
`provider create` сообщат о неподдерживаемом поле.

## Каталоги workspace и global

В workspace профиль имеет приоритет над global-профилем с тем же ID. Для
операций с global-каталогом требуется роль platform-admin; запись в workspace
доступна его admin или owner.

```bash
quarshell --workspace team-ml profile import -f codex-api.yaml
quarshell profile list --workspace team-ml
quarshell profile export codex-api --global -o yaml
```
