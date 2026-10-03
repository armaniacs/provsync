# Backups and Undo

## How It Works

- Just before every write, all affected files are backed up as one operation.
- Location: `~/.local/state/provsync/` — `index.json` (operation history) and `backups/<op-id>/` (SHA-256-verified file snapshots).
- The last 20 operations are retained; older ones are pruned automatically. `PROVSYNC_KEEP` changes the retention count (an integer of 1 or more).

## Viewing History

```console
$ provsync undo --list
20261002T093045-8c2d  2026-10-02 09:30:45  push opencode
    /home/you/.config/opencode/opencode.json
20261002T093012-3fa1  2026-10-02 09:30:12  pull kilocode
    /home/you/.config/provsync/config.json
```

## Restoring

- `provsync undo` restores the last write; `provsync undo <id>` restores a specific operation. It applies directly, without `--write`.
- undo records the pre-restore state as a new operation and prints an ID to redo with (`redo: provsync undo <id>`).
- `provsync undo --prune --keep <n>` removes history, keeping the newest n operations (the removed count is printed).

## --no-backup Behavior

Writes made with `--no-backup` are recorded as marker operations. A subsequent `undo` warns that the latest write was made without a backup and that it restores the state before that write.

## Permissions

- Newly created central configs and state directories are created with 0600 / 0700 permissions. Existing file permissions are never changed.
- `status` suggests `chmod` when permissions are loose.

## Symlinks

- Symlinks from dotfiles management are preserved. Writes go to the link target's real file; the link itself is never replaced by a regular file.
- Broken links error out before writing. `list` shows the link target as ` (symlink → target)`.

## Exclusive Lock

- The `--write` and `undo` restore sections are protected by an exclusive lock (flock) on the state directory. Concurrent runs wait, then fail with "another provsync is running (...)" after a 10-second timeout.
- Preview, `diff`, and `status` take no lock.
- A crashed process recovers automatically because the OS releases the lock.
