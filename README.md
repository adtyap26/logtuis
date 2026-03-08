# logtuis

A lightweight, fast terminal log viewer. Single binary, no dependencies. Supports plain and gzip-compressed log files.

## Install

```bash
go build -o logtuis .
```

Or download the binary from releases and put it in your `$PATH`.

## Usage

```bash
# open current directory
logtuis

# open a specific directory
logtuis /var/log/redis
```

## Key Bindings

### File List

| Key       | Action                            |
| --------- | --------------------------------- |
| `j` / `↓` | Move down                         |
| `k` / `↑` | Move up                           |
| `g`       | Jump to top                       |
| `G`       | Jump to bottom                    |
| `/`       | Search / fuzzy filter by filename |
| `esc`     | Clear search                      |
| `enter`   | Open selected file                |
| `q`       | Quit                              |

### Log Viewer

| Key       | Action                              |
| --------- | ----------------------------------- |
| `j` / `↓` | Scroll down one line                |
| `k` / `↑` | Scroll up one line                  |
| `ctrl+d`  | Scroll down half page               |
| `ctrl+u`  | Scroll up half page                 |
| `g`       | Jump to top                         |
| `G`       | Jump to bottom                      |
| `/`       | Search pattern in log (like grep)   |
| `n`       | Next match                          |
| `N`       | Previous match                      |
| `esc`     | Clear search / go back to file list |
| `q`       | Go back to file list                |

## Supported File Formats

- `*.log` — plain text logs
- `*.log.gz` — gzip compressed logs
- `*.log.1.gz`, `*.log.2.gz`, ... — rotated and compressed logs (e.g. Redis, Nginx, syslog)

## License

GPL-3.0 — see [LICENSE](LICENSE).
