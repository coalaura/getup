![banner](.github/banner.png)

A zero-friction tool for streaming compressed remote backups that leverages your existing SSH environment to eliminate redundant configuration.

## Features

- **Streaming Compression**: Supports zstd, gzip, xz, lz4, bzip2, zip or uncompressed streams.
- **SSH Native**: Leverages your existing `~/.ssh/config` and `~/.ssh/known_hosts`.
- **Files or Commands**: Back up selected files as a tar archive or the output of any remote command.
- **Atomic Writes**: Streams into a `.partial` file and only exposes the completed backup after an atomic rename.
- **Multiple Tasks per Host**: Define several backup jobs against the same SSH host, each with its own `name`, source and pre/post scripts.
- **Optional Tasks**: Disable jobs from automatic runs while keeping them available for explicit invocation.
- **Exclusion Support**: Easily exclude specific directories or files using the `!` prefix.
- **Real-time Stats**: Shows write speed and total progress during the transfer.
- **Optional Encryption**: Encrypt your backups with a password using [age](https://github.com/FiloSottile/age).

## Installation

You can bootstrap **getup** with a single command. This script will detect your OS and CPU (`amd64`/`arm64`), download the correct binary and install it to `/usr/local/bin/getup`.

```bash
curl -sL https://src.ws2.sh/getup/install.sh | sh
```

### Binary
Download the latest prebuilt version for your platform from the [releases](https://github.com/coalaura/getup/releases/latest) page.

### Build from source
Requires Go 1.25+.

```bash
git clone https://github.com/coalaura/getup.git
cd getup
go build -o getup .
```

## Configuration

The tool looks for a YAML configuration file at `~/.config/getup.yml`.

```yaml
password: "" # Optional password to encrypt backups using age
tasks:
  - server: web-server            # Matches entry in ~/.ssh/config
    name: web-files               # Used for the archive filename (falls back to server if omitted)
    target: /local/backups        # Local directory to store archives
    compression: zstd             # Optional; defaults to zstd
    compression-level: 3          # Optional; uses the algorithm's native level
    pre:                          # Optional commands to run before backup
      - "systemctl stop nginx"
      - "docker pause myapp"
    post:                         # Optional commands to run after backup
      - "docker unpause myapp"
      - "systemctl start nginx"
    files:
      - /etc/nginx                # Include directory
      - /var/www
      - "!/var/www/cache"         # Exclude directory (prefix with !)
      - /root/.bashrc             # Include specific file

  - server: web-server            # Same host, different job
    name: web-mysql
    disabled: true                # Skip unless explicitly requested by name
    target: /local/backups
    pre:
      - "systemctl stop mysql"
    post:
      - "systemctl start mysql"
    files:
      - /var/lib/mysql

  - server: web-server
    name: web-mysql-dump
    target: /local/backups
    command: "mysqldump --all-databases" # Compressed command output; cannot be combined with files
```

| Field               | Required | Description |
|---------------------|----------|-------------|
| `server`            | yes      | Hostname as defined in `~/.ssh/config` |
| `target`            | yes      | Local directory for the resulting archive |
| `name`              | no       | Job label used in the archive filename; defaults to `server` |
| `disabled`          | no       | Skip the task when running without names; defaults to `false` |
| `files`             | yes      | Paths to include; prefix with `!` to exclude; if empty `command` is required |
| `command`           | yes      | Command whose standard output is backed up; cannot be combined with `files`; if empty `files` is required |
| `compression`       | no       | `none`, `zstd`, `gzip`, `xz`, `lz4`, `bzip2` or `zip`; defaults to `zstd` |
| `compression-level` | no       | Native level for the selected algorithm; defaults to 3 (zstd), 6 (gzip/xz/zip), 1 (lz4) or 9 (bzip2) |
| `pre`               | no       | Commands run on the remote host before the backup |
| `post`              | no       | Commands run on the remote host after the backup |

## Usage

Run with no arguments to process every task in your config:

```bash
getup
```

Pass one or more task names to run only those jobs (matched case-insensitively against each task's `name` or `server` if `name` is omitted):

```bash
getup web-files
getup web-files web-mysql
```

Tasks are still run in the order they appear in the config. Unknown names are ignored. Tasks with `disabled: true` are skipped when `getup` is run without names, but run normally when explicitly requested.

### Output Format
File archives are saved using the format `{name}-{timestamp}.tar.{extension}`. Command output is saved as `{name}-{timestamp}.{extension}`. Uncompressed file tasks use `.tar`; uncompressed command tasks have no extension.

If `name` is omitted, `server` is used instead. With a password set, `.age` is appended to either extension. Backups are written with an additional `.partial` suffix and atomically renamed when complete.

## Requirements

- **Local**: `getup` binary.
- **Remote**: `bash` and the configured compression command must be available in the shell path. File-based tasks also require `tar`.
