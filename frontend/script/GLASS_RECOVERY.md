# Recovering an interrupted Glass theme migration

`upgrade-glass-theme.mjs --apply` validates the installed theme, stages a complete copy, and keeps the old directory as `Glass.backup-<timestamp>-<id>`. Its two directory renames are **not** an atomic exchange. If the process or host dies after the first rename, `/opt/komari/data/theme/Glass` may be temporarily absent. Do not run another migration or reinstall the theme in that state.

Eligibility checks pin the manifest, HTML and one node-card chunk, **not** every JS/CSS file. Other files are carried into the staged copy unchanged. Review customizations in the complete installed theme before `--apply`; an eligible dry-run is not proof the whole asset tree is unmodified.

1. Stop only the panel service if it is still running, and inspect `/opt/komari/data/theme/` for the specific `Glass.backup-*` directory created by the interrupted run. Do not use a wildcard as a destination; there may be older backups. Check that `Glass` is absent, that the chosen backup is a real directory and contains `komari-theme.json`, `dist/index.html`, and the referenced fingerprinted chunk. Compare their SHA-256 digests with one of the pinned sets in `upgrade-glass-theme.mjs` before recovery. If the backup was customized or no pinned set matches, stop and investigate rather than guessing.
2. With the exact verified backup path assigned to `backup`, restore the old theme in the same filesystem:

   ```sh
   theme_dir=/opt/komari/data/theme/Glass
   backup=/opt/komari/data/theme/Glass.backup-EXACT-TIMESTAMP-ID
   test ! -e "$theme_dir" && test ! -L "$theme_dir" && test -d "$backup" && test ! -L "$backup" || exit 1
   mv -T -- "$backup" "$theme_dir"
   ```

3. Verify the restored manifest/index/chunk digests, owner and mode, then restart only the panel service and verify the public page and assets over HTTP. Leave any `.Glass-stage-*` directory in place until the restored service is healthy; remove a confirmed orphan only after review. If `Glass` already exists, **do not** overwrite it with a backup—determine whether the new stage was installed and verify it first.

This recovery procedure is a manual contingency, not authorization to change a production installation. Do not run `--apply`, stop/restart services, or restore a backup without explicit deployment/recovery authorization.
