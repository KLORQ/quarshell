---
status: draft
---

# План MicroVM: parity, production и релиз

Этот план ведёт текущий libkrun MicroVM путь к функциональному соответствию
OpenShell v0.1.4 и production-релизу. Он дополняет
[описание MicroVM](microvm.md) и [runtime acceptance matrix](../reference/runtime-acceptance.md).
Статус пункта меняется только после реализации и проверки на целевой платформе.

## Текущее состояние

Локальный драйвер уже умеет создавать и восстанавливать VM, импортировать OCI
rootfs и tar, хранить persistent overlay, выполнять команды и PTY, отдавать
логи, копировать файлы, проксировать policy и передавать SSH direct-tcpip.
Поддержан двухдисковый bootstrap/workload путь: явный
`OPENSHELL_VM_BOOTSTRAP_IMAGE` или доверенный `OPENSHELL_SANDBOX_IMAGE` как
bootstrap, отдельный read-only workload disk и persistent overlay. Gateway
пока блокирует создание VM до host supervisor/isolation и acceptance.

Открытые ограничения parity inventory:

- `vm.bootstrap-image` — двухдисковый путь и digest recovery проходят macOS
  acceptance; в guest init добавлены ветки `image-prep` для versioned tar и
  OCI layout payload. Host stages tar/OCI layout, запускает prep worker на
  pinned bootstrap, проверяет ext4 через debugfs и только затем публикует
  workload disk. Live macOS acceptance проходит; статус остаётся `partial` до
  Linux KVM и полного отказного/security набора.
- `vm.gateway-create-ready` — сквозной Gateway create/start/ready на VM заблокирован.
- Linux KVM amd64/arm64, установленная поставка/upgrade/rollback и GPU/VFIO
  требуют целевых хостов; GPU/VFIO и PID limits пока не поддерживаются.
- Открытые/частичные пункты остальных `vm.*` записаны в
  `quarshell-driver-vm/vm/testdata/openshell-v0.1.4-parity.json`.

## Этап 1. Зафиксировать контракт и parity

- [x] Сверить каждый `vm.*` ID с upstream v0.1.4 и закрепить минимальную
  совместимую семантику: create/start/stop/delete, recovery, overlay, identity,
  bootstrap, OCI/tar, exec/PTY, logs/events, SSH forwarding и GPU/resource
  capabilities.
- [x] Для bootstrap определить отдельные поля конфигурации и их приоритет:
  bootstrap image — системное окружение агента и supervisor; workload image —
  пользовательский rootfs. Запретить неявное использование workload image как
  bootstrap fallback.
- [x] Перенести двухдисковую схему OpenShell: bootstrap root disk, отдельный
  read-only workload image disk в `/image-rootfs`, persistent writable overlay
  для sandbox. Подготовку образа выполнять через bootstrap prep VM; проверять
  наличие `/image-rootfs` и guest agent digest перед публикацией disk в
  immutable cache. Реализация проходит macOS ARM64/HVF live acceptance;
  Linux KVM acceptance остаётся отдельным release gate.
- [x] Сохранить в durable state bootstrap/workload digests и ссылки на оба
  immutable cache entry; recovery должен проверять оба артефакта, держать оба
  cache lease и не загружать новый image по mutable tag.
- [x] Проверить порядок bootstrap/workload/overlay disks fake worker-тестом;
  live OCI acceptance подтверждает mount workload image, writable persistent
  `/sandbox` overlay через Stop/Start и recovery с тем же pinned digest после
  перемещения image tag. Unit tests подтверждают отказ для отсутствующего,
  усечённого и повреждённого ext4 workload disk. Linux KVM acceptance остаётся
  отдельным обязательным gate.
- [x] До запуска VM проверять ext4 superblock bootstrap/workload disks; тесты
  подтверждают отказ при отсутствующем, усечённом и повреждённом workload disk;
  guest mount и сохранение overlay проверены live OCI acceptance на macOS ARM64.
- [ ] Довести lifecycle extension registry до полной parity. Уже реализованы
  и покрыты тестами registry, детерминированный порядок и обратный cleanup,
  активация только зарегистрированными доверенными labels, валидация
  descriptor/launch plan и required backend features (guest-init-dropins
  поддержан, unsupported features отклоняются), приватное
  состояние расширений, runtime environment без записи в durable state,
  ConfigureLaunch/BeforeLaunch/BeforeRestore/AfterRestore/Stopped/ProcessExited/
  AfterDelete hooks и исполняемые guest-init drop-ins через overlay. Live
  acceptance на macOS ARM64 проверяет label activation и выполнение drop-in.
  Ошибка ConfigureLaunch/digest/BeforeRestore при восстановлении теперь
  запускает обратную очистку extensions с причиной `restore-failed`; ошибка
  AfterRestore тоже запускает очистку. Регрессионные unit tests проверяют оба
  пути. Остаётся расширить capability resolver для новых backend features и
  проверить отказные сценарии на Linux KVM.
- [x] Покрыть bootstrap fallback, отказ при несовместимой OCI platform,
  восстановление по digest после перемещения tag и отсутствие env/credentials в
  durable state (`TestDefaultHostConfigUsesTrustedDefaultImageAsBootstrapFallback`,
  `TestOCIImageIndexRejectsMissingPlatformDescriptor`, live OCI acceptance,
  `TestCreateWithBootstrapImagePersistsSeparatePinnedDisks`).
- [x] Зафиксировать OpenShell семантику `rootfs_tar_path` вместе с `image`:
  tar становится workload filesystem, а image ref сохраняется как запрошенная
  identity. `TestValidateSpecAllowsRootfsTarWithImageSelector`, Create unit test
  и live acceptance покрывают этот путь; более ранний GPU отказ остаётся
  приоритетным.
- [x] Проверять, что workload/proxy environment и secrets отсутствуют в
  durable state (`TestCreateWithBootstrapImagePersistsSeparatePinnedDisks`).
- [x] Обновить parity JSON доказательствами новых unit и live acceptance tests;
  сохранять `partial` до выполнения оставшихся parity и target gates.

Ожидаемый контракт конфигурации после реализации:

```go
type MicroVMImages struct {
    Bootstrap OCIImageRef // pinned linux/amd64 или linux/arm64
    Workload  OCIImageRef // sandbox image; независим от bootstrap
}

func (c MicroVMImages) Validate() error {
    if c.Bootstrap.Digest == "" {
        return errors.New("MicroVM bootstrap image must be pinned by digest")
    }
    if c.Workload.Digest == "" {
        return errors.New("MicroVM workload image must be pinned by digest")
    }
    return nil
}
```

Это целевая форма API, не утверждение о существующей реализации. Сначала
согласовать её с текущим `driver_config`, Gateway API и upstream поведением.

### P0: bootstrap prep VM (первый обязательный вертикальный срез)

До Gateway и расширений закрыть подготовку workload image тем же способом, что
OpenShell. Сегодня `vm.go` вызывает host-side `prepareRootfsTarWorkloadDisk` /
`prepareOCIWorkloadDiskFromResolved`, а `populateRootfsFromOCI` применяет слои
на хосте. Это сохраняет digest проверки, но не обеспечивает upstream границу
подготовки в VM.

Целевой интерфейс подготовки должен отделять staging, запуск и публикацию:

```go
type PrepInput struct {
    BootstrapDisk string // immutable, digest pinned
    PayloadDisk   string // OCI layout или rootfs tar; без registry credentials
    OutputDisk    string // временный ext4, только для этого запуска
    SourceDigest  string
    Platform      string // linux/amd64 или linux/arm64
}

func prepareWorkloadInGuest(ctx context.Context, in PrepInput) (string, error) {
    if err := validatePrepInput(in); err != nil { return "", err }
    if err := runPrepWorker(ctx, in, "QUARSHELL_VM_INIT_MODE=image-prep"); err != nil {
        return "", err
    }
    if err := verifyExt4Directory(in.OutputDisk, "/image-rootfs"); err != nil {
        return "", fmt.Errorf("guest image-prep did not produce image-rootfs: %w", err)
    }
    return publishImmutableCache(ctx, in.OutputDisk, in.SourceDigest, in.Platform)
}
```

Порядок реализации и проверки:

1. Добавить versioned payload format для tar и verified OCI layout. Registry
   auth остаётся только на host pull; в payload идут blobs/manifest и source
   identity, без env, token или credential helper данных.
2. Добавить guest init `image-prep`: смонтировать payload read-only, output
   disk writable, развернуть во временный `image-rootfs.partial`, применить
   ownership/runtime настройки, затем атомарно переименовать в
   `image-rootfs`, `sync`, unmount и завершить.
3. Запустить выделенный libkrun worker с тем же pinned bootstrap disk; передать
   режим через allowlisted env. Учесть cancellation, timeout, console tail,
   process-group cleanup и удалить staging при любом отказе.
4. Не доверять одному exit code: проверить ext4 и наличие обязательных файлов,
   права/метаданные и source identity до атомарной cache publication. Только
   затем записать workload digest в durable state.
5. Включить формат payload и guest init digest в cache key; использовать cache
   lock/lease и проверять публикацию при конкурентной сборке.

Минимальные тестовые примеры для этого среза:

```go
func TestPrepPublishesOnlyVerifiedImageRootfs(t *testing.T) { /* fake worker + ext4 verifier */ }
func TestPrepCancellationRemovesStagingAndDoesNotCache(t *testing.T) { /* cancel worker */ }
func TestPrepPayloadNeverContainsRegistryCredentials(t *testing.T) { /* scan staged payload */ }
```

Сначала unit/fake-worker tests на каждом PR; затем live acceptance на macOS
ARM64/HVF и Linux KVM amd64/arm64. Пока хотя бы один target lane не прошёл,
`vm.bootstrap-image` остаётся `partial`, а release gate закрыт.

## Этап 2. Закончить host supervisor и изоляцию

- [ ] Запускать Gateway-managed worker через host supervisor с закрытым
  аутентифицированным control channel, ограниченными RPC и управляемым shutdown.
  Обычный VM worker и image-prep worker теперь получают закрытый parent-liveness
  pipe и завершают свою process group вместе с broker при аварийном выходе
  Gateway. Это закрывает путь orphan worker, но не заменяет отдельный
  managed supervisor и host filesystem isolation.
- [ ] На Linux применять private user/mount namespaces, private mount
  propagation, dedicated non-root host identity и явный UID/GID map; проверять
  их до старта broker/VM.
- [ ] На macOS ARM64 запускать подписанный и проверенный worker bundle через
  отдельный supervisor path. Gateway preflight теперь требует
  `QUARSHELL_VM_WORKER_TEAM_ID`, проверяет подпись через `codesign --verify --strict`
  и совпадение Team ID; managed supervisor и live Gateway acceptance ещё не
  реализованы.
- [ ] Не передавать host workspace mount. Хранить workspace в persistent guest
  overlay и проверять изоляцию host filesystem/network/vsock.
- [ ] Обеспечить отзыв токенов, rotation/retry, process-group cleanup и запрет
  возрождения завершённого workload.
- [ ] Оставить Gateway fail-closed, пока supervisor и обе целевые платформы не
  пройдут обязательные acceptance.

Проверка capability перед публикацией:

```go
if !hostIsolation.Ready() || !hostSupervisor.Ready() {
    return status.Error(codes.FailedPrecondition,
        "MicroVM host isolation is not ready")
}
```

## Этап 3. Gateway сквозной путь

- [ ] Реализовать Gateway create → driver create → start → guest readiness →
  Get/List status с observed workload identity.
- [ ] Проверить cancel создания, timeout, клиентский disconnect и cleanup при
  ошибке на каждом переходе.
- [ ] Проверить stop/delete, рестарт Gateway с running/stopped intent,
  потерю worker/broker, повторный Start и терминальный exit.
- [ ] Проверить extension activation только после policy-controlled label,
  unknown extension rejection и очистку extension state при delete.
- [ ] Проверить `WatchSandbox` logs/events, replay tail и закрытие stream.
- [ ] Проверить interactive exec stdin/output/resize/signals/exit status,
  copy-in/out, policy proxy и SSH TCP forwarding через публичные CLI/API.
- [ ] Проверить rootfs upload ограничения, OCI auth, digest pinning и отказ при
  перемещении tag во время recovery.

## Этап 4. Матрица тестов до релиза

**На каждом изменении:**

```sh
cd quarshell-driver && go test ./...
cd ../quarshell-gateway && go test ./...
cd ../quarshell-runtime && go test ./...
cd .. && task vm:parity-check compat:inventory docs:strict
```

**Локальный live acceptance на macOS Apple Silicon/HVF:**

```sh
task vm:live-darwin
```

Проверяет реальный worker/guest путь, lifecycle/recovery, OCI digest/auth,
PTY, identity, policy и SSH TCP roundtrip. Успех на macOS не заменяет Linux KVM
или Gateway acceptance.

**Обязательные целевые CI runners:**

```sh
# Linux amd64 и Linux arm64 runners с /dev/kvm и настроенным runtime/rootfs
task vm:live-linux

# Сквозной публичный Gateway/CLI тест с VM provider
QUARSHELL_VM_GATEWAY_ACCEPTANCE=1 task vm:gateway-acceptance
```

`vm:live-linux` запускает существующий KVM acceptance script для amd64/arm64.
Workflow `quarshell-driver/.github/workflows/live-vm-kvm.yml` теперь запускает
оба native KVM lane на push и pull request; в настройках защиты ветки нужно
сделать оба job обязательными и настроить self-hosted runners с меткой
`quarshell-kvm`. Gateway task
`vm:gateway-acceptance` пока отсутствует: его нужно реализовать и включить в CI.
Все driver test packages локально cross-compiled для `linux/amd64` и
`linux/arm64` (`GOOS=linux GOARCH=... CGO_ENABLED=0 go test -run '^$' -exec
/usr/bin/true ./...`). Это подтверждает компиляцию Linux-путей, но не исполняет
тесты и не заменяет KVM acceptance.
Обязательные gates должны завершаться ошибкой, а не `skip`, если host
capability отсутствует.

**Ресурсные и отказные проверки:** создание/удаление при заполнении cache,
ограничение CPU/RAM, повторный старт после kill worker, повреждённый state,
отсутствующий KVM/HVF, недоступный registry, неверные digest и отказ namespace.
GPU/VFIO запускать только на выделенном совместимом runner; до этого явно
advertise capability как unsupported.

## Этап 5. Установка, обновление и откат

- [ ] Собирать отдельные подписанные артефакты для Linux amd64/arm64 и macOS
  arm64; фиксировать версии libkrun, guest agent, worker и layout state.
- [ ] Публиковать checksums, SBOM, provenance и инструкции проверки подписи.
- [ ] Прогнать чистую установку без локальных Go replacements и unpublished
  зависимостей; затем upgrade с предыдущей стабильной версии.
- [ ] Проверить миграцию state на копии данных, несовместимую версию state,
  interruption миграции и rollback. Миграция должна быть атомарной или
  сохранять восстанавливаемую резервную копию.
- [ ] Проверить install/upgrade/uninstall на чистых Linux amd64/arm64 и macOS
  ARM64 машинах; убедиться, что uninstall не удаляет пользовательские VM data
  без явного действия оператора.
- [ ] Выпустить canary, проверить ошибки старта/изоляции/восстановления и только
  затем продвигать тот же digest артефактов в stable.

## Критерий разрешения релиза

Релиз разрешён, когда обязательные parity IDs реализованы либо явно объявлены
вне заявленного профиля продукта; Gateway create-ready и supervisor/isolation
прошли acceptance на macOS ARM64 и Linux KVM amd64/arm64; install/upgrade/
rollback пройдены на опубликованных артефактах; тесты, security checks, docs и
parity inventory зелёные. GPU/VFIO можно исключить из первого профиля только
если продукт не заявляет GPU и capability не рекламируется.
