# Arch Linux packaging

Two PKGBUILDs, mirroring the repo's CLI/GUI split:

- `productivio/` — the CLI (`cmd/pomodoro`), zero dependencies, `CGO_ENABLED=0`.
  Installs the binary as `pomodoro`.
- `productivio-gui/` — the Fyne GUI (`gui/cmd/pomodoro-gui`), cgo, links
  against X11/Mesa via `depends=()`. Installs the binary as `pomodoro-gui`.

The AUR package name is `productivio`/`productivio-gui`, not `pomodoro` —
`pomodoro` is already taken on the AUR by an unrelated project (checked via
the AUR RPC search API). `productivio` matches this repo's Go module path
and was free at the time of writing.

Both build from the `v$pkgver` git tag's release tarball, not `-git` HEAD.

## Before these are usable

The tarball URL points at `v1.0.0`, but that tag only exists locally — it has
not been pushed to `origin` yet. `sha256sums` for that source is `SKIP` until
then. Once the tag is pushed:

```bash
cd packaging/archlinux/productivio && updpkgsums
cd ../productivio-gui && updpkgsums   # only replaces the first (tarball) hash
```

`updpkgsums` downloads the source and fills in the real checksum in place.

## Building and testing locally (on Arch, or an Arch container)

```bash
cd packaging/archlinux/productivio      # or productivio-gui
makepkg -si                             # build + install, pulling in dependencies
namcap PKGBUILD                         # lint the PKGBUILD
namcap *.pkg.tar.zst                    # lint the built package
```

`productivio-gui` needs a real X/Wayland + Mesa environment to build and run;
it won't work in a minimal chroot with no display libraries installed. Its
`depends` list (X11/Mesa/fontconfig libs) is hand-derived from what Fyne
links against, not verified with `namcap` on real Arch hardware yet — treat
it as a starting point and let `namcap *.pkg.tar.zst` correct it.

## Publishing to the AUR

This repo only carries the PKGBUILDs; publishing them is a separate,
account-bound step this assistant can't do on your behalf:

1. Create the AUR packages `productivio` and `productivio-gui` (an SSH key on
   your AUR account is required; re-check name availability first at
   `aur.archlinux.org/packages?K=<name>` in case it's been taken since).
2. In each package's own directory here, generate `.SRCINFO`:
   ```bash
   makepkg --printsrcinfo > .SRCINFO
   ```
3. Push `PKGBUILD` + `.SRCINFO` (+ the `.desktop` file, for `pomodoro-gui`)
   to `ssh://aur@aur.archlinux.org/<pkgname>.git`.

Bumping `pkgver` after a new tag means: update `pkgver`, reset `pkgrel=1`,
`updpkgsums`, regenerate `.SRCINFO`, and push again.
