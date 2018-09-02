SHA := $(shell git rev-parse --short=8 HEAD)
BUILDDATE := $(shell date --rfc-3339=seconds)

.PHONY: build test image push test directipserver

build: directipserver

directipserver:
	cd cmd/$@ && \
	GOOS=linux CGO_ENABLED=0 go build -ldflags "-X 'main.revision=$(SHA)' -X 'main.builddate=$(BUILDDATE)'" -o bin/$@

image:
	docker build -f Dockerfile -t globalsafetrack/directip:g$(SHA) -t globalsafetrack/directip:latest .

push:
	docker push globalsafetrack/directip:g$(SHA)
	docker push globalsafetrack/directip:latest

test:
	go test -v -race -cover
