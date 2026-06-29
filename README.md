# Vigil

Vigil ist ein leichtgewichtiger CLI-Prozessmanager – eine PM2-Alternative für Linux, die auf systemd und nginx aufsetzt.

Verwaltet Node.js- und Static-Apps durch Erzeugung von systemd-Units bzw. nginx-Site-Konfigurationen.

## Installation

```bash
go install github.com/chris576/vigil/cmd/vigil@latest
```

Oder nach Clonen des Repos:

```bash
go build -o vigil ./cmd/vigil
sudo mv vigil /usr/local/bin/
```

**Voraussetzungen:**

- Linux mit systemd (für Node-Apps)
- nginx installiert (für Static-Apps)
- Go 1.22+

## Kurzstart

### Node-App registrieren

```bash
sudo vigil add my-api \
  --type node \
  --entry /opt/myapp/server.js \
  --port 3000 \
  --working-dir /opt/myapp \
  --env-file /opt/myapp/.env
```

### Static-App registrieren

```bash
sudo vigil add my-site \
  --type static \
  --build-dir /opt/mysite/dist \
  --port 8080 \
  --nginx-domain example.com \
  --nginx-path /var/www/example
```

### Status prüfen

```bash
vigil list
```

### App starten/stoppen/neustarten

```bash
vigil start my-api
vigil stop my-api
vigil restart my-api
```

### App entfernen

```bash
vigil remove my-api
```

---

## Kommandos im Detail

### `vigil add [name]`

Registriert eine neue App.

**Flags:**

| Flag | Typ | Pflicht | Beschreibung |
|------|-----|---------|-------------|
| `--type` | `string` | ja* | `node` oder `static` |
| `--port` | `int` | ja* | Port der App |
| `--entry` | `string` | bei `node` | Einstiegsskript (z.B. `server.js`) |
| `--build-dir` | `string` | bei `static` | Build-Verzeichnis (z.B. `dist/`) |
| `--working-dir` | `string` | nein | Arbeitsverzeichnis |
| `--env-file` | `string` | nein | Pfad zur Environment-Datei |
| `--nginx-domain` | `string` | nein | nginx `server_name` |
| `--nginx-path` | `string` | nein | nginx `root`-Pfad |
| `--config` | `string` | nein | Pfad zur ecosystem.json |
| `--force` | `bool` | nein | Überschreibt existierende App |

\* `--type` und `--port` sind nur Pflicht, wenn ohne `--config` gearbeitet wird.

**Beispiele:**

```bash
# Einfache Node-App
vigil add my-api --type node --entry app.js --port 3000

# Static-App mit nginx-Domain
vigil add my-site --type static --build-dir dist --port 8080 --nginx-domain example.com --nginx-path /var/www/example

# Mit Arbeitsverzeichnis und Env-File
vigil add my-api --type node --entry server.js --port 4000 --working-dir /app --env-file /app/.env

# Aus ecosystem.json (alle Apps)
vigil add --config ecosystem.json

# Nur eine bestimmte App aus ecosystem.json
vigil add my-api --config ecosystem.json

# Vorhandene App überschreiben
vigil add my-api --type node --entry app.js --port 3000 --force
```

---

### `vigil remove <name>`

Entfernt eine registrierte App inklusive systemd-Unit bzw. nginx-Site-Konfiguration.

```bash
vigil remove my-api
```

**Aktion:** Stoppt die App, deaktiviert die Unit/Site, löscht die Konfigurationsdateien und entfernt den Eintrag aus dem Store.

---

### `vigil list`

Listet alle registrierten Apps auf.

```bash
vigil list
```

**Ausgabe:**
```
my-api               node    port 3000    active
my-site              static  port 8080    active
```

---

### `vigil start <name>`

Startet eine registrierte App.

```bash
vigil start my-api
```

- **Node-Apps:** Startet die systemd-Unit
- **Static-Apps:** Erzeugt die nginx-Site-Konfiguration und lädt nginx neu

---

### `vigil stop <name>`

Stoppt eine laufende App.

```bash
vigil stop my-api
```

- **Node-Apps:** Stoppt die systemd-Unit
- **Static-Apps:** Entfernt den nginx-Site-Symlink und lädt nginx neu

---

### `vigil restart <name>`

Startet eine App neu.

```bash
vigil restart my-api
```

- **Node-Apps:** Führt `systemctl restart` aus
- **Static-Apps:** Deaktiviert und aktiviert die nginx-Site neu

---

### `vigil init`

Generiert eine `ecosystem.json`-Vorlage.

```bash
vigil init
vigil init --output mein-projekt.json
```

**Erzeugt:**
```json
{
  "name": "my-app",
  "type": "node",
  "port": 3000,
  "entry": "./app.js",
  "build_dir": "",
  "env_file": "",
  "working_dir": "",
  "nginx_domain": "",
  "nginx_path": "",
  "created_at": "2025-01-01T00:00:00Z",
  "enabled": true
}
```

---

### `vigil version`

Zeigt die installierte Version an.

```bash
vigil version
```

---

## Ecosystem-JSON (ecosystem.json)

Die `ecosystem.json` erlaubt es, mehrere Apps auf einmal zu registrieren. Das Format ist an PM2 angelehnt.

### Einzelner Prozess

```json
{
  "name": "my-api",
  "type": "node",
  "port": 3000,
  "entry": "./app.js",
  "build_dir": "",
  "env_file": "/opt/myapp/.env",
  "working_dir": "/opt/myapp",
  "nginx_domain": "",
  "nginx_path": "",
  "enabled": true
}
```

### Mehrere Prozesse (apps-Array)

```json
{
  "apps": [
    {
      "name": "api",
      "type": "node",
      "entry": "server.js",
      "port": 3000,
      "working_dir": "/opt/api",
      "env_file": "/opt/api/.env"
    },
    {
      "name": "frontend",
      "type": "static",
      "build_dir": "/opt/frontend/dist",
      "port": 8080,
      "nginx_domain": "example.com",
      "nginx_path": "/var/www/example"
    },
    {
      "name": "admin",
      "type": "static",
      "build_dir": "/opt/admin/build",
      "port": 8081,
      "nginx_domain": "admin.example.com",
      "nginx_path": "/var/www/admin"
    }
  ]
}
```

### JSON-Felder

| Feld | Typ | Pflicht | Beschreibung |
|------|-----|---------|-------------|
| `name` | `string` | **ja** | Name der App (eindeutig) |
| `type` | `string` | **ja** | `"node"` oder `"static"` |
| `port` | `int` | **ja** | Port (muss > 0 sein) |
| `entry` | `string` | bei `node` | Einstiegsskript (z.B. `"app.js"`) |
| `build_dir` | `string` | bei `static` | Build-Verzeichnis (z.B. `"dist"`) |
| `env_file` | `string` | nein | Pfad zur `.env`-Datei |
| `working_dir` | `string` | nein | Arbeitsverzeichnis der App |
| `nginx_domain` | `string` | nein | nginx `server_name` |
| `nginx_path` | `string` | nein | nginx `root`-Pfad |
| `enabled` | `bool` | nein | Ob die App aktiv ist (default: `false`) |

### Nutzung

```bash
# Alle Apps aus der Datei registrieren
vigil add --config ecosystem.json

# Eine bestimmte App aus der Datei registrieren
vigil add api --config ecosystem.json
vigil add frontend --config ecosystem.json
```

Fehlerhafte Apps werden übersprungen (mit Warnung), die restlichen werden registriert.

---

## Architektur

```
cmd/vigil/main.go
  └─ internal/cli/           ← Cobra-CLI
       └─ internal/process/  ← Manager + Store + Process-Typen
            ├─ internal/systemd/  ← DBus-Client (Node-Apps)
            └─ internal/nginx/    ← Site-Config-Management (Static-Apps)
```

### Komponenten

| Komponente | Aufgabe |
|---|---|
| **CLI** (`internal/cli/`) | Cobra-Commands, Context-Injection |
| **Process** (`internal/process/`) | Manager-Logik, JSON-Store, Validierung |
| **systemd** (`internal/systemd/`) | DBus-Verbindung für systemd-Unit-Lifecycle |
| **nginx** (`internal/nginx/`) | nginx-Site-Konfiguration (sites-available/-enabled) |

---

## Funktionsweise

### Node-Apps (type: "node")

Vigil erzeugt eine systemd-Unit-Datei unter `/etc/systemd/system/<name>.service`:

```ini
[Unit]
Description=Vigil: my-api
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/myapp
ExecStart=/usr/bin/node /opt/myapp/server.js
Restart=on-failure
RestartSec=5
EnvironmentFile=/opt/myapp/.env

[Install]
WantedBy=multi-user.target
```

Die Unit wird via DBus aktiviert und gestartet. systemd übernimmt das Restart-Verhalten, Logging (`journalctl`) und Prozess-Isolation.

### Static-Apps (type: "static")

Vigil erzeugt eine nginx-Site-Konfiguration unter `/etc/nginx/sites-available/<name>.conf`:

```nginx
server {
    listen 8080;
    server_name example.com;
    root /var/www/example;
    index index.html;
}
```

Ein Symlink `/etc/nginx/sites-enabled/<name>.conf` → `sites-available/<name>.conf` aktiviert die Site. nginx wird neu geladen.

---

## Speicherort

Jede App wird als einzelne JSON-Datei gespeichert. Schreibvorgänge sind atomar (Temp-Datei + `os.Rename`).

| Benutzer | Speicherpfad |
|----------|-------------|
| **root** (UID 0) | `/etc/vigil/apps/<name>.json` |
| **Non-Root** | `~/.config/vigil/apps/<name>.json` |

**Beispiel `/etc/vigil/apps/my-api.json`:**

```json
{
  "name": "my-api",
  "type": "node",
  "port": 3000,
  "entry": "/opt/myapp/server.js",
  "build_dir": "",
  "env_file": "/opt/myapp/.env",
  "working_dir": "/opt/myapp",
  "nginx_domain": "",
  "nginx_path": "",
  "created_at": "2025-01-15T10:30:00Z",
  "enabled": true
}
```

---

## Fehlerbehandlung

- **`add` mit `--config`:** Fehlerhafte Apps werden übersprungen. Am Ende wird die Anzahl der erfolgreichen und fehlgeschlagenen Registrierungen ausgegeben. Bei mindestens einem Fehler gibt der Befehl einen Exit-Code `!= 0` zurück.
- **Doppelte Apps:** Ohne `--force` wird `add` einen Fehler ausgeben, wenn die App bereits existiert.
- **Validierung:** Vor dem Speichern wird jedes `Process`-Objekt validiert (Pflichtfelder je Type).
- **Atomare Writes:** Der Store schreibt in eine Temp-Datei und führt dann `os.Rename` aus – bei Absturz während des Schreibens bleibt die alte Konfiguration erhalten.

---

## nginx-Troubleshooting

Falls nginx nach `vigil start` / `vigil stop` nicht neu lädt:

```bash
# nginx-Konfiguration testen
nginx -t

# Manuell neu laden
nginx -s reload

# Status prüfen
systemctl status nginx
```

Vigil ruft `nginx -s reload` auf. Schlägt dies fehl (weil z.B. die Konfiguration fehlerhaft ist), wird der Fehler zurückgegeben.
