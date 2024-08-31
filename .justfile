default:
  @just --list

builddir := "build"
projname := "redr"

build:
  mkdir -p {{builddir}}/current-target
  go build -o {{builddir}}/current-target/{{projname}}

build-target OS ARCH:
  mkdir -p {{builddir}}/{{OS}}/{{ARCH}}
  GOOS={{OS}} GOARCH={{ARCH}} go build -o {{builddir}}/{{OS}}/{{ARCH}}/{{projname}}{{ if OS == "windows" { ".exe" } else { "" } }}

build-all: \
  (build-target "windows" "amd64") \
  (build-target "windows" "arm64") \
  (build-target "linux" "amd64")   \
  (build-target "linux" "arm64")   \
  (build-target "darwin" "amd64")  \
  (build-target "darwin" "arm64")

run *ARGS:
  go run . {{ARGS}}

clean:
  rm -rf {{builddir}}

