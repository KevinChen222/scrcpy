# Third-party components

This is a community fork of Genymobile/scrcpy, not an official Genymobile release.

- **scrcpy v5.0.1** — Genymobile and Romain Vimont, Apache-2.0. The official Windows or Linux runtime archive is redistributed in `runtime/`, including its license. Source and corresponding release: https://github.com/Genymobile/scrcpy/tree/v5.0.1 and https://github.com/Genymobile/scrcpy/releases/tag/v5.0.1.
- **ADB, SDL, FFmpeg, libusb and runtime dependencies** — shipped as part of the official scrcpy Windows / Linux distribution. Build recipes, versions and source references are in https://github.com/Genymobile/scrcpy/tree/v5.0.1/release.
- **mygo** — MIT, https://github.com/egoist/mygo, pinned in `go.mod`.
- **purego** — Apache-2.0, https://github.com/ebitengine/purego.
- **go-text/typesetting** — Unlicense or BSD-3-Clause, https://github.com/go-text/typesetting.
- **golang.org/x/image** — BSD-3-Clause, https://go.googlesource.com/image.
- **go-qrcode** — MIT, https://github.com/skip2/go-qrcode. Generates Android wireless-debugging pairing QR codes locally.

The portable package includes each Go dependency's license in `licenses/`.
