#!/usr/bin/env bash
# carryAPI 跨平台发布构建脚本
# 用法: bash scripts/release.sh [版本号,默认 0.2.0-beta]
# 产物输出到 release/ 目录(.gitignore 已忽略,不进入版本库)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RELEASE_DIR="$ROOT/release"
VERSION="${1:-0.2.0-beta}"

echo "==> 构建前端(web/dist)..."
(cd "$ROOT/web" && npm install --no-audit --no-fund >/dev/null 2>&1 || true)
(cd "$ROOT/web" && npm run build)

echo "==> 清理旧产物..."
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

build_one() {
  local os=$1 arch=$2 name
  if [ "$os" = "windows" ]; then
    name="carryapi-${os}-${arch}.exe"
  else
    name="carryapi-${os}-${arch}"
  fi
  echo "==> GOOS=$os GOARCH=$arch -> $name"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w" -o "$RELEASE_DIR/$name" ./cmd/carryapi)
}

build_one windows amd64
build_one windows arm64
build_one linux  amd64
build_one linux  arm64
build_one darwin amd64
build_one darwin arm64

echo "==> 打包..."
pack() {
  local file=$1 base
  base="$(basename "$file")"
  if [[ "$file" == *.exe ]]; then
    if command -v zip >/dev/null 2>&1; then
      (cd "$RELEASE_DIR" && zip -q "${base}.zip" "$base")
    else
      (cd "$RELEASE_DIR" && tar -a -cf "${base}.zip" "$base")
    fi
  else
    (cd "$RELEASE_DIR" && tar -czf "${base}.tar.gz" "$base")
  fi
}
for f in "$RELEASE_DIR"/*; do
  [ -f "$f" ] && pack "$f"
done

echo ""
echo "==> release/${VERSION} 产物:"
ls -lh "$RELEASE_DIR"
