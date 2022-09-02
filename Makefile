EXECUTABLE=mincalc
MINCALC_VERSION = $(shell echo _`git describe --tags | sed 's/mincalc\///'`)
WINDOWS=$(EXECUTABLE)$(MINCALC_VERSION)_windows_amd64
LINUX_AMD64=$(EXECUTABLE)$(MINCALC_VERSION)_linux_amd64
DARWIN_AMD64=$(EXECUTABLE)$(MINCALC_VERSION)_macos_amd64
LINUX_ARM64=$(EXECUTABLE)$(MINCALC_VERSION)_linux_arm64
DARWIN_ARM64=$(EXECUTABLE)$(MINCALC_VERSION)_macos_arm64

build_mincalc_macos_arm64:
	cd mincalc && GOOS=darwin GOARCH=arm64 go build -o out/$(DARWIN_ARM64)
	cd mincalc && mkdir -p release
	cd mincalc && tar -czvf ./release/$(DARWIN_ARM64).tar.gz out/$(DARWIN_ARM64)
	cd mincalc && ls -lisah ./release/$(DARWIN_ARM64).tar.gz


build_mincalc_linux_arm64:
	cd mincalc && GOOS=linux GOARCH=arm64 go build -o out/$(LINUX_ARM64)
	cd mincalc && mkdir -p release
	cd mincalc && tar -czvf ./release/$(LINUX_ARM64).tar.gz out/$(LINUX_ARM64)
	cd mincalc && ls -lisah ./release/$(LINUX_ARM64).tar.gz

build_mincalc_macos_amd64:
	cd mincalc && GOOS=darwin GOARCH=amd64 go build -o out/$(DARWIN_AMD64)
	cd mincalc && mkdir -p release
	cd mincalc && tar -czvf ./release/$(DARWIN_AMD64).tar.gz out/$(DARWIN_AMD64)
	cd mincalc && ls -lisah ./release/$(DARWIN_AMD64).tar.gz

build_mincalc_linux_amd64:
	cd mincalc && GOOS=linux GOARCH=amd64 go build -o out/$(LINUX_AMD64)
	cd mincalc && mkdir -p release
	cd mincalc && tar -czvf ./release/$(LINUX_AMD64).tar.gz out/$(LINUX_AMD64)
	cd mincalc && ls -lisah ./release/$(LINUX_AMD64).tar.gz

build_mincalc_windows_amd64:
	cd mincalc && GOOS=windows GOARCH=amd64 go build -o out/$(WINDOWS).exe
	cd mincalc && mkdir -p release
	cd mincalc && tar -czvf ./release/$(WINDOWS).tar.gz out/$(WINDOWS).exe
	cd mincalc && ls -lisah ./release/$(WINDOWS).tar.gz

create_mincalc_go_gzip:
	tar -zcvf mincalc/release/mincalc.tar.gz mincalc/go.mod mincalc/main.go

build_mincalc: build_mincalc_macos_arm64 build_mincalc_linux_arm64 build_mincalc_macos_amd64 build_mincalc_linux_amd64 build_mincalc_windows_amd64 create_mincalc_go_gzip