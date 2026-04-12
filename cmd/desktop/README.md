# EncodeSwarmr Desktop Manager

A native GUI application for managing an EncodeSwarmr controller. It provides the same
operational view as the embedded web UI but runs as a local desktop binary with no browser
required.

---

## Overview

The Desktop Manager connects to a running EncodeSwarmr controller over its REST API and
WebSocket interface. It is a pure client — it holds no local database and requires no
configuration files. All state lives on the controller; the desktop app simply presents it.

Intended users are operators and administrators who prefer a persistent, native window over
a browser tab, or who work in environments where opening a browser to the controller is
inconvenient.

---

## Features

- Monitor jobs, tasks, agents, sources, and the encode queue in real time
- View per-task logs with search and follow mode
- Submit and cancel encoding jobs
- Browse the NAS file tree via the file manager
- Manage admin resources: templates, variables, webhooks, users, API keys, agent pools,
  path mappings, encoding rules, encoding profiles, watch folders, auto-scaling, schedules,
  plugins, notification channels, and audit exports
- Multi-profile support: save and switch between multiple controller connections
- Session authentication (username + password) or API key authentication
- Live updates via WebSocket — no polling required

---

## Prerequisites

- **Go 1.25+** — the only build dependency. No CGO, no C toolchain.
- A running EncodeSwarmr controller reachable over HTTP/HTTPS.

On Linux, the Gio framework requires the following system libraries at runtime:

```
libwayland-client  libwayland-egl  libGL  libEGL  libxkbcommon
```

Install on Debian/Ubuntu:

```sh
sudo apt install libwayland-dev libgl1 libegl1 libxkbcommon-dev
```

---

## Building

From the repository root:

```sh
make desktop-windows   # produces bin/encodeswarmr-desktop.exe  (Windows, no console window)
make desktop-linux     # produces bin/encodeswarmr-desktop       (Linux amd64)
```

The build sets `-ldflags "-X main.Version=<git-tag>"` automatically. The Windows target
also passes `-H=windowsgui` to suppress the console window.

Cross-compilation works without additional toolchain setup because the project uses zero CGO:

```sh
GOOS=windows GOARCH=amd64 go build -o bin/encodeswarmr-desktop.exe ./cmd/desktop
GOOS=linux   GOARCH=amd64 go build -o bin/encodeswarmr-desktop     ./cmd/desktop
```

---

## Running

Execute the binary directly — no flags or config files are required:

```sh
# Windows
encodeswarmr-desktop.exe

# Linux
./encodeswarmr-desktop
```

The application opens a 1280x800 window (minimum 900x600). On first launch it displays the
login page.

---

## First Launch — Connecting to a Controller

On first launch the login page is empty. Enter:

1. **Controller URL** — the base URL of the controller, e.g. `http://10.0.1.10:8080` or
   `https://encode.example.com`.
2. **Authentication** — choose session or API key mode (see below).
3. Click **Connect**.

The connection details can be saved as a named profile for future launches.

---

## Multi-Profile Support

The Desktop Manager saves connection profiles to the OS user config directory:

| OS      | Path |
|---------|------|
| Windows | `%APPDATA%\encodeswarmr-desktop\profiles.json` |
| Linux   | `~/.config/encodeswarmr-desktop/profiles.json` |
| macOS   | `~/Library/Application Support/encodeswarmr-desktop/profiles.json` |

Each profile stores a name, controller URL, and authentication details. Profiles are
managed from the login page: add, select, or remove entries before connecting.

Profiles are matched by URL — saving a profile with the same URL as an existing one
replaces it in place.

---

## Authentication Modes

### Session (username + password)

Enter a username and password. The app exchanges these for a session cookie via
`POST /auth/login`. The cookie is held in an in-memory cookie jar for the lifetime of the
session and is not persisted to disk.

### API Key

Enter an API key created in the controller admin panel. The key is sent on every request as
the `X-API-Key` header. When saved in a profile the key is stored in plaintext in
`profiles.json` — restrict file permissions accordingly.

---

## Architecture

The Desktop Manager is a **remote client**. It communicates with the controller exclusively
over HTTP and WebSocket — it shares no code with the controller or agent binaries.

| Concern | Detail |
|---------|--------|
| UI framework | [Gio](https://gioui.org) — immediate-mode, pure Go, zero CGO |
| Rendering | GPU-accelerated via Vulkan/Metal/D3D11/OpenGL depending on platform |
| Concurrency | Gio event loop on one goroutine; API calls run in separate goroutines and invalidate the window on completion |
| State sharing | A single `app.State` struct (RWMutex-guarded) holds the active client, WebSocket client, and authenticated user |
| Navigation | Stack-based router (`nav.Router`); sidebar links call `Replace`, detail pages call `Push` |
| Profiles | Persisted to JSON by `profile.Store` using `os.UserConfigDir()` |

### Package Structure

```
cmd/desktop/
  main.go                  Entry point — wires logger, Application, page registry, event loop

internal/desktop/
  app/
    app.go                 Application struct, Gio event loop, sidebar+content layout
    state.go               Shared runtime state (client, user, profile name, colour palette)
    theme.go               Material theme configuration (colours, fonts)
  client/
    client.go              HTTP client, request/response envelope, RFC 9457 error handling
    auth.go                Login, logout, OIDC helpers
    jobs.go                Job CRUD and action calls
    tasks.go               Task detail and log retrieval
    agents.go              Agent list and detail calls
    sources.go             Source registration, analysis triggers, HDR metadata
    queue.go               Queue list and management
    files.go               File manager (NAS browse)
    audio.go               Audio conversion jobs
    metrics.go             Dashboard metrics
    admin.go               Admin resource CRUD (users, API keys, webhooks, etc.)
    ws.go                  WebSocket client for live updates
    types.go               Shared domain types (Job, Agent, Source, Task, …)
  nav/
    router.go              Stack-based page router with Push/Replace/Pop
    sidebar.go             Collapsible navigation sidebar widget
    pages.go               Sidebar page entry definitions
  page/
    register.go            Registers all page factories with the router
    login.go               Login / profile selection page
    dashboard.go           Dashboard with summary metrics
    jobs.go                Job list page
    job_detail.go          Job detail and task list
    sources.go             Source list page
    source_detail.go       Source detail, VMAF results, HDR info
    agents.go              Agent list page
    agent_detail.go        Agent detail, GPU info, log viewer
    tasks.go               Task detail page
    task_detail.go         Task execution detail with log viewer
    queue.go               Encode queue page
    audio.go               Audio conversion page
    flows.go               Automation flows page
    file_manager.go        NAS file browser
    sessions.go            Active session list
    placeholder.go         Generic placeholder for unimplemented pages
    admin/                 Admin sub-pages (one file per resource type)
  profile/
    store.go               Profile persistence (JSON in OS config dir)
  widget/
    table.go               Paginated, sortable data table
    logviewer.go           Scrollable log viewer with search and follow mode
    badge.go               Status badge chip
    chart.go               Simple bar/line chart widget
    dialog.go              Modal dialog helper
    form.go                Form field helpers (text input, select, checkbox)
    progress.go            Progress bar widget
    searchbar.go           Search input with debounce
```

---

## Linux Packaging

### Debian / Ubuntu (.deb)

```sh
make deb-desktop
```

Requires `nfpm` on `$PATH`. The package metadata is defined in `nfpm-desktop.yaml`.
The `.deb` is written to `dist/`.

Install on the target machine:

```sh
sudo dpkg -i dist/encodeswarmr-desktop_<version>_amd64.deb
```

### Arch Linux

```sh
cd deployments/arch
makepkg -si
```

The `PKGBUILD` in `deployments/arch/` builds from source and installs the binary and a
`.desktop` launcher entry.

---

## Windows Installer

An Inno Setup script is provided at `installer/desktop-setup.iss`. To produce a setup
wizard `.exe`:

1. Install [Inno Setup 6](https://jrsoftware.org/isinfo.php).
2. Build the Windows binary first: `make desktop-windows`
3. Open `installer/desktop-setup.iss` in the Inno Setup IDE and click **Compile**, or run:

```cmd
iscc installer\desktop-setup.iss
```

The installer places the binary in `%ProgramFiles%\EncodeSwarmr Desktop\` and creates
Start Menu and Desktop shortcuts.

---

## Screenshots

_Screenshots will be added here once the UI stabilises._
