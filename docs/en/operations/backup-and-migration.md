# Backup and migration

Export/import moves an entire `CONFIG_ROOT` snapshot for the selected role.

It is intended for moving Desired inputs between machines, keeping a rollback snapshot, or cloning a setup.

## Export

Export uses the default `CONFIG_ROOT` and writes an archive to the current directory.

```console
xp2p client export
xp2p server export
```

## Advanced export

- Export a non-default config root:

```console
xp2p client export --config-root <path>
xp2p server export --config-root <path>
```

- Choose output path and format:

```console
xp2p client export --output <path>
xp2p server export --output <path>
```

Supported archive formats depend on the output file extension:

- `.zip`
- `.tar.gz` (or `.tgz`)

When `--output` is omitted, xp2p picks the default format (`.zip` on Windows, `.tar.gz` on non-Windows) and writes `xp2p-<role>-backup-<timestamp>.<ext>` into the current directory.

## Import

```console
xp2p client import --input <archive>
xp2p server import --input <archive>
```

After import, verify the service status and restart the role service if needed.

## Advanced import

- Import into a non-default config root:

```console
xp2p client import --config-root <path> --input <archive>
xp2p server import --config-root <path> --input <archive>
```

Notes:

- Import accepts only Desired inputs (runtime artifacts under `.state/` are rejected).
- Symlinks inside bundles are not supported.
- Import writes an `apply.request` marker for the imported role so the service can re-apply the new Desired inputs.
