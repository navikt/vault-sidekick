
NAME=vault-sidekick
AUTHOR ?= navikt
REGISTRY ?= docker.io
GOVERSION ?= 1.12.0
HARDWARE=$(shell uname -m)
GIT_SHA=$(shell git --no-pager describe --always --dirty)
VERSION ?= $(shell awk '/release =/ { print $$3 }' main.go | sed 's/"//g')-${GIT_SHA}
LFLAGS ?= -X main.gitsha=${GIT_SHA}
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods -nilfunc -printf -rangeloops -shift -structtags -unsafeptr

.PHONY: test authors changelog build docker static release

default: build

build: 
	@echo "--> Compiling the project"
	mkdir -p bin
	go build -ldflags '-w ${LFLAGS}' -o bin/${NAME}

static: 
	@echo "--> Compiling the static binary"
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w ${LFLAGS}' -o bin/${NAME}

build-with-docker:
	@echo "--> Compiling the project"
	${SUDO} docker run --rm \
		-v ${PWD}:/src \
		-w /src \
		-e GOOS=linux \
		golang:${GOVERSION} \
		make static

docker:
	@echo "--> Building the docker image"
	docker build -t ${REGISTRY}/${AUTHOR}/${NAME}:${VERSION} .

docker-save: docker
	docker save ${REGISTRY}/${AUTHOR}/${NAME}:${VERSION} > ${DOCKER_TAR_FILE}

docker-load: 
	docker load -i ${DOCKER_TAR_FILE}

docker-build-push:
	@echo "--> Building a release image"
	@make docker
	@echo "--> Pushing imagee"
	@docker push ${REGISTRY}/${AUTHOR}/${NAME}:${VERSION}

docker-push: 
	@echo "--> Pushing the image to docker.io"
	docker push ${REGISTRY}/${AUTHOR}/${NAME}:${VERSION}

release: static
	mkdir -p release
	gzip -c bin/${NAME} > release/${NAME}_${VERSION}_linux_${HARDWARE}.gz
	rm -f release/${NAME}

clean:
	rm -rf ./bin 2>/dev/null
	rm -rf ./release 2>/dev/null

authors:
	@echo "--> Updating the AUTHORS"
	git log --format='%aN <%aE>' | sort -u > AUTHORS
vet:
	@echo "--> Running go tool vet $(VETARGS) ."
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@go tool vet $(VETARGS) .

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

gofmt:
	@echo "--> Running gofmt check"
	@gofmt -s -l *.go \
      | grep -q \.go ; if [ $$? -eq 0 ]; then \
            echo "You need to runn the make format, we have file unformatted"; \
            gofmt -s -l *.go; \
            exit 1; \
      fi
cover:
	@echo "--> Running go cover"
	@go test --cover

test: 
	@echo "--> Running the tests"
	go test -v
	@$(MAKE) gofmt
	@$(MAKE) vet

changelog: release
	git log $(shell git tag | tail -n1)..HEAD --no-merges --format=%B > changelog
