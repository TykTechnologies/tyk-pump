#!/bin/bash

set -e

export GO111MODULE=on

# print a command and execute it
show() {
 echo "$@" >&2
 eval "$@"
}

fatal() {
 echo "$@" >&2
 exit 1
}

TEST_TIMEOUT=20m

show go vet . || fatal "go vet errored"

GO_FILES=$(find * -name '*.go' -not -path 'vendor/*' -not -name 'bindata.go')

echo "Formatting checks..."

FMT_FILES="$(gofmt -s -l ${GO_FILES})"
if [[ -n ${FMT_FILES} ]]; then
    fatal "Run 'gofmt -s -w' on these files:\n$FMT_FILES"
fi

echo "gofmt check is ok!"

IMP_FILES="$(goimports -l ${GO_FILES})"
if [[ -n ${IMP_FILES} ]]; then
    fatal "Run 'goimports -w' on these files:\n$IMP_FILES"
fi

echo "goimports check is ok!"

for pkg in $(go list github.com/TykTechnologies/tyk-pump/...);
do
    race="-race"
    echo "Testing... $pkg"
    if [[ ${pkg} == *"pumps" ]]; then
        # run pumps tests without race detector until we add correct testing
        race=""
        # run tests twice for tyk-pump/pumps with different MONGO_DRIVER values
        MONGO_DRIVERS=("mgo" "mongo-go")
        for mongo_driver in "${MONGO_DRIVERS[@]}"; do
            echo "Running tests with MONGO_DRIVER=$mongo_driver"
            export MONGO_DRIVER=$mongo_driver
            coveragefile=`echo "$pkg" | awk -F/ '{print $NF}'`
            show go test -timeout ${TEST_TIMEOUT} ${race} --coverprofile=${coveragefile}.cov -v ${pkg}
        done
    else
        coveragefile=`echo "$pkg" | awk -F/ '{print $NF}'`
        show go test -timeout ${TEST_TIMEOUT} ${race} --coverprofile=${coveragefile}.cov -v ${pkg}
    fi
done