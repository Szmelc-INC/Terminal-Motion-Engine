# Maintainer: SilverX <serainox@gmail.com>
pkgname=termo-git
_repo=Terminal-Motion-Engine
pkgver=r0.0000000
pkgrel=1
pkgdesc="Play media (gif/video) in the terminal as ASCII / ANSI art"
arch=('x86_64' 'aarch64')
url="https://github.com/Szmelc-INC/Terminal-Motion-Engine"
license=('MIT')
makedepends=('git' 'go' 'make')
optdepends=('ffmpeg: video input support')
provides=('termo')
conflicts=('termo')
options=('!debug')
source=("git+$url.git")
sha256sums=('SKIP')

pkgver() {
  cd "$_repo"
  ( set -o pipefail
    git describe --long --tags --abbrev=7 2>/dev/null | sed 's/^v//;s/\([^-]*-g\)/r\1/;s/-/./g' ||
    printf "r%s.%s" "$(git rev-list --count HEAD)" "$(git rev-parse --short=7 HEAD)" )
}

prepare() {
  cd "$_repo"
  export GOPATH="$srcdir/gopath" GOFLAGS="-modcacherw"
  go mod download
}

build() {
  cd "$_repo"
  export GOPATH="$srcdir/gopath" CGO_ENABLED=0 GOFLAGS="-trimpath -mod=readonly -modcacherw"
  make build
}

check() {
  cd "$_repo"
  export GOPATH="$srcdir/gopath" CGO_ENABLED=0 GOFLAGS="-mod=readonly -modcacherw"
  make test
}

package() {
  cd "$_repo"
  install -Dm755 termo "$pkgdir/usr/bin/termo"
  install -Dm644 README.md -t "$pkgdir/usr/share/doc/$pkgname/"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
}
