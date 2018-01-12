.PHONY: build build-nodep

build:
	docker run --rm -v ${PWD}:/go/src/github.com/errnoh/k8s/watch -w /go/src/github.com/errnoh/k8s/watch/example golang /bin/bash -c "go get -v github.com/golang/dep/cmd/dep && dep ensure -v && CGO_ENABLED=0 GOOS=linux go build -v -a -installsuffix cgo -ldflags '-w' -o watch-example && ! ldd ./watch-example"
	docker build -t errnoh/watch-example .

build-nodep:
	docker run --rm -v ${PWD}:/go/src/github.com/errnoh/k8s/watch -w /go/src/github.com/errnoh/k8s/watch/example golang /bin/bash -c "CGO_ENABLED=0 GOOS=linux go build -v -a -installsuffix cgo -ldflags '-w' -o watch-example && ! ldd ./watch-example"
	docker build -t errnoh/watch-example .
