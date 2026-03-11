# logtuis

A lightweight, fast terminal log viewer with SSH remote support. Single binary, no dependencies. View local and remote log files side-by-side, grep across all sources, and run shell pipelines — all from your terminal.

![preview](assets/preview.gif)

## Features

- **Source picker** — choose local directory and/or SSH servers on startup
- **SSH remote logs** — browse, preview, and open log files on remote servers
- **Multi-source grep** (`ctrl+f`) — search across local + all SSH sources at once
- **Shell pipeline runner** (`ctrl+s`) — run any shell command across all sources
- **Log level colorizing** — `ERROR`/`ERR` red, `WARN`/`WRN` yellow, `INFO`/`INF` green (toggle `c`)
- **SSH connection status** — colored dot indicator per server in the file list
- **Watch mode** (`W`) — live tail local and SSH log files, auto-reload every 2s
- **Archive** (`V`) — tar.gz selected files including compressed logs
- **Line numbers** (`L` / `Shift+L`) — toggle in both file list and viewer
- **Gzip support** — read `.log.gz` and rotated compressed logs transparently

## Install

**Download binary (recommended)**

Go to [Releases](../../releases) and download the binary for your platform, then put it in your `$PATH`.

```bash
tar -xzf logtuis_v1.0.0_linux_amd64.tar.gz
mv logtuis /usr/local/bin/
```

**Build from source**

```bash
go build -o logtuis .
```

## Configuration

Create `~/.config/logtuis/config.json` to add SSH servers:

```json
{
  "ssh_sources": [
    {
      "name": "redis-prod",
      "host": "10.100.13.13",
      "port": 22,
      "user": "root",
      "identity": "~/.ssh/id_ed25519",
      "password": "",
      "path": "/var/opt/redislabs/log"
    },
    {
      "name": "app-server",
      "host": "10.100.13.14",
      "port": 22,
      "user": "ubuntu",
      "identity": "",
      "password": "secret",
      "path": "/var/log/myapp"
    }
  ]
}
```

- `identity` — path to SSH private key (e.g. `~/.ssh/id_ed25519`). Leave empty to use password or auto-discover keys in `~/.ssh/`
- `password` — used if key auth is not available
- `path` — remote directory to scan for log files

## Usage

```bash
# launch — source picker appears first
logtuis

# open a specific local directory directly
logtuis /var/log
```

On launch, the **source picker** lets you choose which sources to load:

```
 logtuis — select sources

   [✓]  local       /var/log
   [✓]  redis-prod  root@10.100.13.13:/var/opt/redislabs/log
   [ ]  app-server  ubuntu@10.100.13.14:/var/log/myapp

  j/k navigate • space select • a select all • enter confirm • q quit
```

After confirming, the file list shows files from all selected sources with SSH connection status indicators.

## Key Bindings

### Source Picker

| Key     | Action               |
| ------- | -------------------- |
| `j`/`k` | Navigate             |
| `space` | Toggle source        |
| `a`     | Select / deselect all |
| `enter` | Confirm and load     |
| `q`     | Quit                 |

### File List

| Key       | Action                                     |
| --------- | ------------------------------------------ |
| `j` / `↓` | Move down                                  |
| `k` / `↑` | Move up                                    |
| `g`       | Jump to top                                |
| `G`       | Jump to bottom                             |
| `/`       | Fuzzy filter by filename                   |
| `esc`     | Clear search                               |
| `enter`   | Open selected file                         |
| `L`       | Toggle line numbers                        |
| `ctrl+f`  | Grep pattern across all files (local + SSH) |
| `ctrl+s`  | Run shell pipeline across all sources      |
| `ctrl+r`  | Re-open source picker to change selection  |
| `V`       | Enter select mode (archive)                |
| `q`       | Quit                                       |

### Select Mode (`V`)

| Key     | Action                                                 |
| ------- | ------------------------------------------------------ |
| `space` | Toggle selection on current file                       |
| `j`/`k` | Move cursor                                            |
| `enter` | Archive selected files to `logtuis-<timestamp>.tar.gz` |
| `esc`   | Cancel and clear selection                             |

### Global Grep (`ctrl+f`)

| Key     | Action                                   |
| ------- | ---------------------------------------- |
| `tab`   | Toggle case-sensitive / insensitive      |
| `enter` | Run grep across all files (local + SSH)  |
| `esc`   | Cancel                                   |

### Shell Runner (`ctrl+s`)

| Key             | Action                    |
| --------------- | ------------------------- |
| `ctrl+←` / `→` | Jump word left / right    |
| `ctrl+a`        | Go to start of input      |
| `ctrl+e`        | Go to end of input        |
| `ctrl+k`        | Delete to end of line     |
| `ctrl+w`        | Delete word before cursor |
| `enter`         | Run command               |
| `esc`           | Cancel                    |

### Log Viewer

| Key       | Action                               |
| --------- | ------------------------------------ |
| `j` / `↓` | Scroll down one line                 |
| `k` / `↑` | Scroll up one line                   |
| `ctrl+d`  | Scroll down half page                |
| `ctrl+u`  | Scroll up half page                  |
| `g`       | Jump to top                          |
| `G`       | Jump to bottom                       |
| `/`       | Search pattern (supports `\|` OR)    |
| `tab`     | Toggle case-sensitive search         |
| `n`       | Next match                           |
| `N`       | Previous match                       |
| `f`       | Filter — show only matching lines    |
| `e`       | Export matching lines to `.out` file |
| `:`       | Jump to line number                  |
| `L`       | Toggle line numbers                  |
| `c`       | Toggle log level colorizing          |
| `W`       | Toggle watch mode (auto-reload 2s)   |
| `esc`     | Clear search / go back               |
| `q`       | Go back to file list                 |

## Log Level Colorizing

Press `c` in the viewer to toggle inline keyword highlighting:

| Keyword           | Color  |
| ----------------- | ------ |
| `ERROR` / `ERR`   | Red    |
| `WARN`  / `WRN`   | Yellow |
| `INFO`  / `INF`   | Green  |

Only exact uppercase tokens are colored — words like `no_ERROR` or `INFORMATION` are not affected. Search highlighting always takes priority over level colors.

## Multi-Source Grep & Shell

`ctrl+f` and `ctrl+s` run across **all selected sources** — local and SSH — concurrently. Results stream in as they arrive.

```bash
# grep across everything
grep -i "ERROR" *

# time range filter
awk -v s=14:30 -v e=22:00 'split($2,t,":") && t[1]":"t[2]>=s && t[1]":"t[2]<e' * | grep ERROR

# count errors per file
grep -c "ERROR" *
```

Remote commands run in the configured `path` directory on each server.

## SSH Connection Status

The file list header shows a status dot for each SSH source:

```
 Log Viewer
  ● redis-prod   ● app-server
```

- Green `●` — connected and files loaded
- Red `●` — connection failed

## Watch Mode

Press `W` in the viewer to auto-reload the file every 2 seconds. Works for both local and SSH files. The viewer scrolls to the bottom on each reload — useful for live tailing active logs.

## Archive

- Press `V` to enter select mode
- Navigate with `j`/`k` and press `space` to toggle files
- Press `enter` to create `logtuis-<timestamp>.tar.gz` in the current directory
- `.gz` files are included as-is (no double compression)
- Press `esc` to cancel

## Search

- Press `/` to open the search bar
- Use `|` for OR matching: `ERROR|WARN|timeout`
- Press `tab` to toggle case-sensitive / insensitive
- Press `f` to show only matching lines (filter mode)
- Press `e` to export matching lines to a `.out` file

## Supported File Formats

| Format                     | Description                                    |
| -------------------------- | ---------------------------------------------- |
| `*.log`                    | Plain text logs                                |
| `*.txt`                    | Text files                                     |
| `*.out`                    | Exported filter results                        |
| `*.log.gz`                 | Gzip compressed logs                           |
| `*.log.1.gz`, `*.log.2.gz` | Rotated compressed logs (Redis, Nginx, syslog) |
| `*.log.1`, `*.log.2`       | Rotated plain logs                             |

## License

GPL-2.0 — see [LICENSE](LICENSE).
