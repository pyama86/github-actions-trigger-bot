VERSION := $(shell git-semv now | sed -e 's/v//g')
run:
	go run .


## release_major: release nke (major)
release_major: releasedeps
	git semv major --bump

.PHONY: release_minor
## release_minor: release nke (minor)
release_minor: releasedeps
	git semv minor --bump

.PHONY: release_patch
## release_patch: release nke (patch)
release_patch: releasedeps
	git semv patch --bump


build: releasedeps
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o github-actions-trigger main.go

build_image: build
	export DOCKER_CONTENT_TRUST=1
	docker build -t pyama/github-actions-trigger:$(VERSION) .

push_image:
	docker push pyama/github-actions-trigger:$(VERSION)
	docker tag pyama/github-actions-trigger:$(VERSION) pyama/github-actions-trigger:latest
	docker push pyama/github-actions-trigger:latest
test:
	go test github.com/pyama86/github-actions-trigger-bot/...
## release_major: release nke (major)
release_major: releasedeps
	git semv major --bump

.PHONY: release_minor
## release_minor: release nke (minor)
release_minor: releasedeps
	git semv minor --bump

.PHONY: release_patch
## release_patch: release nke (patch)
release_patch: releasedeps
	git semv patch --bump

.PHONY: releasedeps
releasedeps: git-semv

.PHONY: git-semv
git-semv:
ifeq ($(shell uname),Linux)
	which git-semv || (wget https://github.com/linyows/git-semv/releases/download/v1.2.0/git-semv_linux_x86_64.tar.gz && tar zxvf git-semv_linux_x86_64.tar.gz && sudo mv git-semv /usr/bin/)
else
	which git-semv > /dev/null || brew tap linyows/git-semv
	which git-semv > /dev/null || brew install git-semv
endif
