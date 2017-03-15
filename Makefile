package=github.com/TykTechnologies/tyk-pump
distname=tyk-pump
goversion=1.8.0

build:
	docker run -v `pwd`:/go/src/$(package) golang:$(goversion)-alpine go build -o /go/src/$(package)/dist/$(distname) -v $(package)

#optional push to docker register
dockerize: build
	docker build -t tykio/tyk-pump-docker-pub:v0.4.1.1 .
