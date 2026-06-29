# Feature: Logs & Log-Saving

## Beschreibung

Zeigt Logs verwalteter Apps auf der Konsole an (live/follow). Ermoeglicht persistentes Speichern mit automatischer Rotation via logrotate.

Zwei Sub-Features:
- `vigil logs` — Logs streamen (journalctl/tail)
- `vigil logsave` — Persistentes Logging aktivieren/deaktivieren/status

## Architektur

```
internal/cli/logs.go         <- "vigil logs" cobra command
internal/cli/logsave.go      <- "vigil logsave" cobra command (enable/disable/status)

internal/process/manager.go  <- dispatch je nach App-Typ an systemd oder nginx
internal/process/logconfig.go <- LogConfig struct + JSON-Persistenz (LogStore)

internal/systemd/contract.go <- Client interface: Logs, SetupLogging, RemoveLogging
internal/systemd/client.go   <- Implementierung: journalctl, unit file edit, logrotate

internal/nginx/contract.go   <- Client interface: Logs, SetupLogging, RemoveLogging
internal/nginx/client.go     <- Implementierung: tail, cat, logrotate
```

### Typen

```go
// internal/process/logconfig.go
type LogConfig struct {
    Name    string `json:"name"`
    Enabled bool   `json:"enabled"`
    LogPath string `json:"log_path"`
    MaxSize string `json:"max_size"`
    Rotate  int    `json:"rotate"`
}

type LogStore interface {
    Load(name string) (LogConfig, error)
    Save(cfg LogConfig) error
    Delete(name string) error
    List() ([]LogConfig, error)
}
```

### Dispatch-Logik (Manager)

| App-Typ | Log-Quelle | Log-Save Mechanismus |
|---------|-----------|---------------------|
| Node    | `journalctl -u <name>.service` | Unit-Datei: `StandardOutput/Error=append:<path>` + logrotate |
| Static  | `tail/cat /var/log/nginx/<name>.access.log` | logrotate config |

## API

### `vigil logs <name>`

Zeigt App-Logs auf der Konsole.

| Flag | Kurz | Default | Beschreibung |
|------|------|---------|-------------|
| `--lines` | `-n` | `50` | Anzahl vergangener Zeilen (0 = alle) |
| `--follow` | `-f` | `false` | Folgt neuen Log-Zeilen (tail -f) |
| `--output` | `-o` | `""` | Schreibt Ausgabe zusaetzlich in Datei (Tee) |

Node: streamt `journalctl -u <name>.service -o cat [-n N] [-f]`
Static: streamt `tail [-f] -n N /var/log/nginx/<name>.access.log` (bzw. `cat` bei lines=0)

### `vigil logsave enable <name>`

Aktiviert persistentes Log-Speichern.

| Flag | Default | Beschreibung |
|------|---------|-------------|
| `--max-size` | `10M` | Maximale Dateigroesse vor Rotation (z.B. `10M`, `1G`) |
| `--output` | `/var/log/vigil` | Log-Ausgabeverzeichnis |
| `--rotate` | `3` | Anzahl rotierter Logs die behalten werden |

Node: fuegt `StandardOutput=append:/var/log/vigil/<name>.log` und `StandardError=append:/var/log/vigil/<name>.log` in systemd Unit ein, erstellt logrotate config, `daemon-reload` + `restart`.
Static: erstellt logrotate config fuer `/var/log/nginx/<name>.access.log`.

Speichert `LogConfig` als JSON im LogStore.

### `vigil logsave disable <name>`

Deaktiviert persistentes Log-Speichern.

Node: entfernt `StandardOutput=`/`StandardError=` aus Unit-Datei, loescht logrotate config, `daemon-reload` + `restart`.
Static: loescht logrotate config.

Loescht `LogConfig` aus LogStore.

### `vigil logsave status <name>`

Zeigt Status des persistenten Log-Speicherns.

Ausgabe:
```
App:       <name>
Enabled:   true/false
Log Path:  /var/log/vigil/<name>.log
Max Size:  10M
Rotate:    3
```

## Konfiguration

### LogStore (JSON)

Persistenz der LogConfig pro App.

| Bedingung | Pfad |
|-----------|------|
| root (`euid==0`) | `/etc/vigil/logs/<name>.json` |
| user | `~/.config/vigil/logs/<name>.json` |

### systemd Logpfad

`/var/log/vigil/<name>.log` (stdout + stderr appended via `StandardOutput=append:` und `StandardError=append:`)

### nginx Logpfad

`/var/log/nginx/<name>.access.log` (im Site-Template fest codiert)

### logrotate Config

Gespeichert als `/etc/logrotate.d/vigil-<name>`.

Inhalt:
```
<logpath> {
    size <maxSize>
    rotate <rotate>
    compress
    missingok
    notifempty
    copytruncate
}
```

Defaults: `size 10M`, `rotate 3`, `copytruncate` (sicher fuer systemd/nginx).

## Usage

```bash
# Letzte 50 Zeilen anzeigen
vigil logs my-app

# Alle Zeilen anzeigen
vigil logs my-app -n 0

# Folgen (tail -f)
vigil logs my-app -f

# Logs in Datei schreiben (Tee)
vigil logs my-app -o /tmp/debug.log

# Persistente Log-Speicherung aktivieren (Node)
vigil logsave enable my-app --max-size 50M --rotate 7

# Status anzeigen
vigil logsave status my-app

# Speicherung deaktivieren
vigil logsave disable my-app
```

## Abhaengigkeiten

| Package | Verwendung |
|---------|-----------|
| `github.com/spf13/cobra` | CLI commands & flags |
| `github.com/coreos/go-systemd/v22/dbus` | systemd D-Bus (unit start/stop/reload) |
| `journalctl` (systemd) | Log-Stream fuer Node-Apps |
| `tail` / `cat` (coreutils) | Log-Stream fuer Static-Apps |
| `logrotate` | Log-Rotation (wird Konfig geschrieben, nicht direkt aufgerufen) |
| `os/exec` | Ausfuehrung von journalctl, tail, cat, systemctl |
