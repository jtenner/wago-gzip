# Third-party notices

This project uses Go's standard-library `compress/gzip` and `compress/flate`
packages. Their source is not vendored. The Go distribution's complete license
and patent grant are reproduced without modification in
[`third_party/go.LICENSE`](third_party/go.LICENSE) and
[`third_party/go.PATENTS`](third_party/go.PATENTS).

This project's Wago integration compiles against `github.com/wago-org/wago`,
which is Apache-2.0. The root [`LICENSE`](LICENSE) contains that license's full
text, and Wago's upstream attribution notice is preserved without modification
in [`third_party/wago.NOTICE`](third_party/wago.NOTICE).

Wago depends on `golang.org/x/sys`, which is distributed under the Go Authors'
BSD-style license and patent grant. The pinned module's files are preserved
without modification in [`third_party/x-sys.LICENSE`](third_party/x-sys.LICENSE)
and [`third_party/x-sys.PATENTS`](third_party/x-sys.PATENTS).
