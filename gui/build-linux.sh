#!/usr/bin/env bash
set -euo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
version="${1:-$(cat -- "$root/VERSION")}"
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-\.][a-zA-Z0-9\.]+)?$ ]] || { echo 'Invalid GUI version' >&2; exit 1; }
[[ "$(uname -m)" == x86_64 ]] || { echo 'Portable runtime currently supports Linux x86_64' >&2; exit 1; }
export CGO_ENABLED=0
# Keep toolchain/module downloads and build caches inside this project.
export GOMODCACHE="${GOMODCACHE:-$root/../.tools/linux-go-mod}"
export GOCACHE="${GOCACHE:-$root/../.tools/linux-go-cache}"
cd "$root"
mkdir -p build
runtime_version=v5.0.1
runtime_hash=9f969d30cc574816077edecda65719c1c70b0aee1df1ba5fa00779ac07b8fd6e
archive="$root/build/scrcpy-linux-x86_64-$runtime_version.tar.gz"
if [[ ! -f "$archive" ]]; then
    curl --fail --location --retry 2 --output "$archive.partial" "https://github.com/Genymobile/scrcpy/releases/download/$runtime_version/scrcpy-linux-x86_64-$runtime_version.tar.gz"
    mv -- "$archive.partial" "$archive"
fi
printf '%s  %s\n' "$runtime_hash" "$archive" | sha256sum --check --status
# This builds only the launcher; the scrcpy client/server are official binaries.
go test -buildvcs=false .
go vet -buildvcs=false .
name="scrcpy-lan-$version-linux-x64"
package="$root/build/$name"
# Module-cache licenses can be read-only. Replace the generated package as a
# whole so repeated builds also discard stale files from earlier dependencies.
if [[ -e "$package" || -L "$package" ]]; then
    rm -rf -- "$package"
fi
mkdir -p "$package/runtime" "$package/licenses"
go build -trimpath -buildvcs=false -tags mygo_noinspector -ldflags "-s -w -X main.version=$version" -o "$package/scrcpy-lan" .
tar -xzf "$archive" --strip-components=1 -C "$package/runtime"
cp assets/icon.png "$package/icon.png"
cp README-linux.md "$package/README.md"
cp README.md "$package/Windows-and-common.md"
cp ../LICENSE THIRD_PARTY_NOTICES.md "$package/"
# Enumerate modules actually linked into the GUI, including native dependencies.
# Save first so a failed Go command cannot be hidden by process substitution.
go list -deps -f '{{with .Module}}{{.Path}}|{{.Dir}}{{end}}' . | sort -u > "$root/build/module-licenses.txt"
while IFS='|' read -r module dir; do
    [[ -n "$module" && -n "$dir" ]] || continue
    for license in LICENSE LICENSE.md LICENSE.txt NOTICE; do
        if [[ -f "$dir/$license" ]]; then
            cp -- "$dir/$license" "$package/licenses/${module//\//_}-$license"
        fi
    done
done < "$root/build/module-licenses.txt"
cat > "$package/start.sh" <<'LAUNCHER'
#!/usr/bin/env bash
set -euo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -z "${DISPLAY:-}" ]]; then
    echo '需要 X11 桌面或启用了 XWayland 的 Wayland 桌面（DISPLAY 未设置）。' >&2
    exit 1
fi
export GDK_BACKEND=x11
exec "$root/scrcpy-lan" "$@"
LAUNCHER
chmod +x "$package/start.sh" "$package/scrcpy-lan" "$package/runtime/scrcpy" "$package/runtime/adb"
printf '{"gui":"%s","scrcpy":"%s","runtime_sha256":"%s"}\n' "$version" "$runtime_version" "$runtime_hash" > "$package/versions.json"
tar -czf "build/$name.tar.gz" -C build "$name"
(cd build && sha256sum "$name.tar.gz" > "$name.sha256")
# Switch the desktop's stable launcher only after packaging succeeds.
ln -s -- "$name" "build/.current-linux-$$"
mv -Tf -- "build/.current-linux-$$" "build/current-linux"
printf 'Linux GUI: %s/start.sh\nPackage: %s/build/%s.tar.gz\n' "$package" "$root" "$name"
