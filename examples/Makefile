APP_NAME = aqi
APP_PATH = ./

# build dist
BUILD_PATH := ./dist

# build at
BUILD_DATE = $(shell date +'%F %T')

# git versions
GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT = $(shell git rev-list --count HEAD)
GIT_REVISION = $(shell git rev-parse --short HEAD)
GIT_COMMITAT = $(shell git --no-pager log -1 --format="%at")

# flags
FLAGS_PKG = github.com/wonli/aqi
LDFLAGS = "-X '$(FLAGS_PKG).BuildDate=$(BUILD_DATE)' \
		   -X '$(FLAGS_PKG).Branch=$(GIT_BRANCH)' \
		   -X '$(FLAGS_PKG).CommitVersion=$(GIT_COMMIT)' \
		   -X '$(FLAGS_PKG).Revision=$(GIT_REVISION)' \
		   -extldflags '-static -s -w'"

# params
GO_FLAGS = -ldflags $(LDFLAGS) -trimpath -tags netgo

# Go build fn 1-GOOS 2-GOARCH 3-FILE EXT
define go/build
	GOOS=$(1) GOARCH=$(2) CGO_ENABLED=0 go build $(GO_FLAGS) -o $(BUILD_PATH)/$(APP_NAME)-$(1)-$(2)-latest$(3) ${APP_PATH}
endef

# PHONY
.PHONY: darwin linux windows

darwin:
	$(call go/build,darwin,amd64)
	$(call go/build,darwin,arm64)

linux:
	$(call go/build,linux,amd64)

windows:
	$(call go/build,windows,amd64,.exe)

linux64:
	$(call go/build,linux,amd64)