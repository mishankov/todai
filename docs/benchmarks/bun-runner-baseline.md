# Bun runner production baseline

This migration changes the production runner from interpreted JavaScript plus installed
dependencies to a standalone Bun executable. Image comparison remains part of deployment issue
#17 because this repository does not yet contain that image.

## Reproducible measurements

Use the same Linux host and architecture for both candidates. Record five cold starts and five
steady-state fake-runtime runs:

```shell
/usr/bin/time -v ./dist/pi-runner < smoke.jsonl > events.jsonl
du -h ./dist/pi-runner
```

For the former implementation, measure the compiled JavaScript entry together with its production
dependency directory and runtime. For the Bun implementation, measure only `dist/pi-runner` and
the Go backend. Record:

- compressed and uncompressed image size;
- runner artifact size;
- time from process spawn to `runner.ready`;
- peak resident memory for a fake run;
- time from `run.start` to `run.completed`.

## Migration result

Correctness does not depend on an improvement in these values. The CI architecture matrix executes
the standalone fake-runtime protocol smoke test on native Linux x64 and arm64 runners. The final
container numbers will be added by #17 when its backend image is available.

The migration was sampled locally on 2026-07-23 on macOS arm64. Startup is the elapsed time from
spawn to `runner.ready`; warm startup is the median of four runs after the first run. The fake-run
measurement includes process startup and one complete protocol run.

| Runner | Artifact measured | First startup | Warm startup | Fake run | Maximum RSS |
| --- | ---: | ---: | ---: | ---: | ---: |
| Former JavaScript entry on the separately installed runtime | 12 KiB entry, excluding runtime and dependencies | 468 ms | 358 ms | 420 ms | 165 MiB |
| Standalone Bun 1.3.14 executable | 74 MiB | 1,536 ms | 151 ms | 190 ms | 132 MiB |

The former payload size is intentionally not presented as an image comparison: its runtime and
production dependency tree were not isolated from development dependencies in this checkout. #17
must report the two complete compressed and uncompressed backend image sizes after assembling the
otherwise identical images.
