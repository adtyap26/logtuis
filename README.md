# logtuis

A lightweight, fast terminal log viewer. Single binary, no dependencies. Supports plain, rotated, and gzip-compressed log files.

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

## Usage

```bash
# open current directory
logtuis

# open a specific directory
logtuis /var/log/redis
```

## Key Bindings

### File List

| Key       | Action                        |
| --------- | ----------------------------- |
| `j` / `↓` | Move down                     |
| `k` / `↑` | Move up                       |
| `g`       | Jump to top                   |
| `G`       | Jump to bottom                |
| `/`       | Fuzzy filter by filename      |
| `esc`     | Clear search                  |
| `enter`   | Open selected file            |
| `ctrl+f`  | Grep pattern across all files |
| `ctrl+r`  | Reload / rescan directory     |
| `V`       | Enter select mode             |
| `q`       | Quit                          |

### Select Mode (V)

| Key     | Action                                      |
| ------- | ------------------------------------------- |
| `space` | Toggle selection on current file            |
| `j`/`k` | Move cursor up/down                         |
| `enter` | Archive selected files to `logtuis-<timestamp>.tar.gz` |
| `esc`   | Cancel and clear selection                  |

### Global Grep (ctrl+f)

| Key     | Action                              |
| ------- | ----------------------------------- |
| `tab`   | Toggle case-sensitive / insensitive |
| `enter` | Run grep across all log files       |
| `esc`   | Cancel                              |

### Log Viewer

| Key       | Action                               |
| --------- | ------------------------------------ |
| `j` / `↓` | Scroll down one line                 |
| `k` / `↑` | Scroll up one line                   |
| `ctrl+d`  | Scroll down half page                |
| `ctrl+u`  | Scroll up half page                  |
| `g`       | Jump to top                          |
| `G`       | Jump to bottom                       |
| `/`       | Search pattern in log (like grep)    |
| `tab`     | Toggle case-sensitive search         |
| `n`       | Next match                           |
| `N`       | Previous match                       |
| `f`       | Filter — show only matching lines    |
| `e`       | Export matching lines to `.out` file |
| `:`       | Jump to line number                  |
| `W`       | Toggle watch mode (auto-reload 2s)   |
| `esc`     | Clear search / go back to file list  |
| `q`       | Go back to file list                 |

## Archive

- Press `V` to enter select mode
- Navigate with `j`/`k` and press `space` to toggle each file
- Press `enter` to create a `logtuis-<timestamp>.tar.gz` in the current directory
- `.gz` files are included as-is (no double compression)
- Press `esc` to cancel without archiving

## Search

- Press `/` to open the search bar
- Use `|` for OR matching: `ERROR|WARN|service_log` — like `grep -E`
- Press `tab` while searching to toggle between **case-insensitive** (default) and **case-sensitive**
- Press `enter` to apply — all matching sub-patterns are highlighted
- Press `f` to hide all non-matching lines (filter mode)
- Press `e` to export matching lines to a `.out` file in the current directory
- Exported `.out` files are visible in the file list and can be opened in the viewer

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
