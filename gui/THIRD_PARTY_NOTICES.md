# Third-party components

This is a community fork of Genymobile/scrcpy, not an official Genymobile release.

- **scrcpy v5.0** — Genymobile and Romain Vimont, Apache-2.0. The Windows runtime archive is redistributed unmodified in `runtime/`, including its `LICENSE.txt`. Source and corresponding release: https://github.com/Genymobile/scrcpy/tree/v5.0 and https://github.com/Genymobile/scrcpy/releases/tag/v5.0.
- **ADB, SDL, FFmpeg, libusb and runtime dependencies** — shipped as part of the official scrcpy Windows distribution. Build recipes, versions and source references are in https://github.com/Genymobile/scrcpy/tree/v5.0/release.
- **mygo** — MIT, https://github.com/egoist/mygo, pinned in `go.mod`.
- **purego** — Apache-2.0, https://github.com/ebitengine/purego.
- **go-text/typesetting** — Unlicense or BSD-3-Clause, https://github.com/go-text/typesetting.
- **golang.org/x/image** — BSD-3-Clause, https://go.googlesource.com/image.

The portable package includes each Go dependency's license in `licenses/`.
