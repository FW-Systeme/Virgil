# Feature: Process-Manager

## Beschreibung

Vigil ist ein leichtgewichtiger CLI-Prozessmanager (PM2-Alternative). Verwaltet Node.js- und Static-Apps via systemd (Node) und nginx (Static). Bietet API zum Hinzufuegen, Entfernen, Starten, Stoppen, Neustarten und Auflisten von Apps.

## Architektur

### 3-Schichten

```
cmd/vigil/main.go
  └─ internal/cli/        ← Cobra-CLI, injiziert *process.Manager via Context
       └─ internal/process/  ← Manager, Store, Process-Typen
            ├─ internal/systemd/  ← DBus-Client fuer systemd-Units
            └─ internal/nginx/    ← nginx Site-Config-Management
```

### Package-Struktur

| Package | Pfad | Aufgabe |
|---|---|---|
| `cli` | `internal/cli/` | Cobra-Commands, Context-Injection |
| `process` | `internal/process/` | Manager, Store, Process-Typ, Ecosystem-Parser |
| `systemd` | `internal/systemd/` | DBus-Client fuer Unit-Lifecycle |
| `nginx` | `internal/nginx/` | Site-Config-Erstellung/Aktivierung |

### Wichtige Typen

```go
// process/contract.go
type Type string
const TypeNode   Type = "node"
const TypeStatic Type = "static"

type Process struct {
    Name        string    `json:"name"`
    Type        Type      `json:"type"`
    Port        int       `json:"port"`
    Entry       string    `json:"entry,omitempty"`     // Node: Einstiegsskript
    BuildDir    string    `json:"build_dir,omitempty"`  // Static: Build-Ordner
    EnvFile     string    `json:"env_file,omitempty"`
    WorkingDir  string    `json:"working_dir,omitempty"`
    NginxDomain string    `json:"nginx_domain,omitempty"`
    NginxPath   string    `json:"nginx_path,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    Enabled     bool      `json:"enabled"`
}

type Store interface {
    Load(name string) (Process, error)
    Save(p Process) error
    Delete(name string) error
    List() ([]Process, error)
    AppPath(name string) (string, error)
}

type Manager struct { /* store + systemd.Client + nginx.Client */ }
```

## API

### process.Manager

| Methode | Signatur | Beschreibung |
|---|---|---|
| `New` | `New(Store, systemd.Client, nginx.Client) *Manager` | Konstruktor |
| `AddProcess` | `AddProcess(ctx, Process, force bool) error` | Speichert + erzeugt Unit/Site |
| `RemoveProcess` | `RemoveProcess(ctx, name string) error` | Stoppt + deaktiviert + loescht |
| `StartProcess` | `StartProcess(ctx, name string) error` | Startet Unit/Site |
| `StopProcess` | `StopProcess(ctx, name string) error` | Stoppt Unit/Site |
| `RestartProcess` | `RestartProcess(ctx, name string) error` | Restartet Unit/Site |
| `Status` | `Status(ctx, name string) (activeState, subState string, err error)` | Liefert Status |
| `ListProcesses` | `ListProcesses(ctx) ([]Process, error)` | Listet alle gespeicherten Prozesse |

### process.Process

| Methode | Signatur | Beschreibung |
|---|---|---|
| `Validate` | `Validate() error` | Prueft Pflichtfelder je Type |

### Weitere

| Funktion | Signatur | Beschreibung |
|---|---|---|
| `ParseEcosystemFile` | `ParseEcosystemFile(r io.Reader) ([]Process, error)` | Parst PM2-kompatibles JSON |
| `NewStore` | `NewStore() (Store, error)` | Erzeugt `jsonFileStore` |

### systemd.Client Interface

```go
StartUnit(ctx, name) error
StopUnit(ctx, name) error
RestartUnit(ctx, name) error
EnableUnit(ctx, name) error
DisableUnit(ctx, name) error
UnitStatus(ctx, name) (activeState, subState string, err error)
CreateUnitFile(name string, content []byte) error
RemoveUnitFile(name string) error
Reload(ctx) error
Close() error
```

### nginx.Client Interface

```go
EnableSite(name string, port int, domain, root string) error
DisableSite(name string) error
RemoveSiteConfig(name string) error
SiteEnabled(name string) (bool, error)
Reload(ctx) error
Close() error
```

### CLI-Befehle

| Befehl | Args | Flags | Beschreibung |
|---|---|---|---|
| `vigil add [name]` | optional name | `--type` (node\|static), `--port`, `--entry`, `--build-dir`, `--config` (ecosystem.json), `--force` | App registrieren |
| `vigil remove <name>` | name (exakt 1) | — | App entfernen |
| `vigil list` | — | — | Alle Apps auflisten |
| `vigil start <name>` | name (exakt 1) | — | App starten |
| `vigil stop <name>` | name (exakt 1) | — | App stoppen |
| `vigil restart <name>` | name (exakt 1) | — | App neustarten |
| `vigil init` | — | `--output` (default: ecosystem.json) | Template generieren |
| `vigil version` | — | — | Version ausgeben |

## Konfiguration

### Store-Pfade

| Bedingung | Basis-Pfad |
|---|---|
| Root (UID 0) | `/etc/vigil/apps/` |
| Non-Root | `~/.config/vigil/apps/` |

Jeder Process wird als `<name>.json` gespeichert. Atomare Writes via `os.CreateTemp` + `os.Rename`.

### systemd-Unit-Template (Node-Type)

```
[Unit]
Description=Vigil: <Name>
After=network.target

[Service]
Type=simple
WorkingDirectory=<WorkingDir>
ExecStart=/usr/bin/node <Entry>
Restart=on-failure
RestartSec=5
EnvironmentFile=<EnvFile>   // nur wenn gesetzt

[Install]
WantedBy=multi-user.target
```

Generierte Datei: `/etc/systemd/system/<name>.service`

### nginx-Site-Template (Static-Type)

```
server {
    listen <Port>;
    server_name <Domain>;
    root <Root>;
    index index.html;
}
```

Generierte Datei: `/etc/nginx/sites-available/<name>.conf`
Symlink: `/etc/nginx/sites-enabled/<name>.conf`

## Usage

### Node-App registrieren

```bash
vigil add my-api \
  --type node \
  --entry /app/server.js \
  --port 3000 \
  --working-dir /app \
  --env-file /app/.env
```

### Static-App registrieren

```bash
vigil add my-site \
  --type static \
  --build-dir /app/dist \
  --port 8080 \
  --nginx-domain example.com \
  --nginx-path /var/www/example
```

### Aus Ecosystem-Datei

```bash
# ecosystem.json:
# { "apps": [
#   {"name":"api","type":"node","entry":"app.js","port":3000},
#   {"name":"web","type":"static","build_dir":"dist","port":8080}
# ] }

vigil add --config ecosystem.json
vigil add api --config ecosystem.json  # nur eine App
```

### Lifecycle

```bash
vigil start my-api
vigil stop my-api
vigil restart my-api
vigil remove my-api
vigil list
```

### Template generieren

```bash
vigil init                     # → ecosystem.json
vigil init --output my-apps.json
```

## Abhaengigkeiten

| Dependency | Verwendung |
|---|---|
| `github.com/spf13/cobra` | CLI-Framework |
| `github.com/coreos/go-systemd/v22` | DBus-Verbindung zu systemd |
| `github.com/stretchr/testify` | Tests (nur Test) |
| `github.com/godbus/dbus/v5` | indirekt ueber go-systemd |

## Teststrategie

- `systemd.Client` + `nginx.Client` als Interfaces → Mock-Objekte in Manager-Tests
- `process.Store` als Interface → Mock-Store + echte `jsonFileStore`-Tests mit temp dirs
- CLI-Tests: `process.Manager` via Context injiziert, Cobra `Execute()` aufgerufenen
- Coverage >85%
- systemd-DBus-Methoden: nil-conn-Tests (kein echter DBus in CI)
- Store: atomare Writes, Permission-Errors, korrupte JSONs, nicht-JSON-Dateien
