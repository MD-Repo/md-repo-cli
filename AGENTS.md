# md-repo-cli

The `mdrepo` command-line tool contributors run. Go + cobra. This is the only
part of MDRepo that runs on a contributor's own machine, so its error messages
are the product for anyone submitting data.

## Commands

```console
make build          # -> bin/mdrepo, version stamped from VERSION.txt
go test ./...
```

Subcommands: `submit`, `get`, `submitls`, `upgrade`.

## Things to know

- **This tool writes the manifest the whole pipeline depends on.**
  `mdrepo-submission.completed.json` records every file's size and MD5;
  `check_new_simulations.py` probes for it and `mdr-process` verifies against it.
  Changing its shape touches three repositories.
- **The status filename is a template** — `mdrepo-submission.%s.json` for
  `unknown`, `inprogress`, `errored`, `completed`. An iRODS *write* ticket cannot
  delete, move or rename, so the cleanup code is commented out and status files
  **accumulate**. Readers must take the newest by iRODS modify time.
- **Server-side metadata validation is blocking.** `submit` posts every
  simulation's TOML to `/api/v1/verify_metadata` before uploading a single byte,
  and aborts on any invalid response. Bad TOML never reaches iRODS.
- **Local validation is a subset.** `ValidateFiles` parses only
  `lead_contributor_orcid`, the three file-name fields and `additional_files`. It
  knows nothing about temperature ranges, software versions or engine format
  combinations — those are the server's job.
- **Directories must be flat.** `ValidateSubmissionSourcePath` rejects a
  simulation directory containing any subdirectory, and requires a non-empty
  `mdrepo-metadata.toml`. `submit` descends exactly one level looking for
  candidates.
- **`validSourcePaths` is sorted, and tickets are consumed in index order.** That
  is what makes a resumed upload land in the same collection. Do not change the
  ordering.
- **Resume is a differential skip, not a byte-range resume.** A file is skipped
  when size matches and the iRODS checksum equals the local MD5, unless `--force`.
- **Connection defaults are hardcoded to prod** — `data.cyverse.org`, zone
  `iplant`, user `anonymous`, landing under `/iplant/home/shared/mdrepo/prod/`.
  `--svc_url` overrides the API base only; staging relies on the ticket carrying
  an absolute staging path.

## Known defects

Both are documented in `../USER-EXPERIENCE.md` with suggested fixes.

- **`submitls` cannot print.** In `printStatusFile`, `statusFilePath` is
  initialised to `""` and never assigned, so the guard below always returns early
  and the download/render block is unreachable. It compiles because `latestObjs`
  is read inside the loop.
- **The final status write discards its error.** In `processTicket`, the
  `inprogress` write is error-checked but the deferred write of the `completed` or
  `errored` marker is not. A failed marker write leaves an upload that printed
  `transfer finished...`, exited 0, and will never be processed — with no record
  anywhere that it exists.
- **The size limit disagrees with the server.** `ParseSize("40GB")` uses binary
  units (40 × 1024³ = 42,949,672,960) while `mdr-process` uses
  40 × 10⁹ = 40,000,000,000. Submissions in that ~2.95 GB window pass here and
  fail server-side after a full upload.
