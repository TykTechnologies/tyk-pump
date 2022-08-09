#!/bin/bash  

set -eao pipefail

function usage {
    local progname=$1
    cat <<EOF
Usage:

    $progname  tag [gwtag]

    tag   -  The tag for the pump image, that needs to be tested.
    gwtag -  (Optional) The  tag for the GW image, that needs to be tested against the pump.
             If not given, the default version would be used, which is $DEFAULT_GW_TAG

Brings up the given tyk gw, and tyk pump and checks if the csv pump is
working properly.
Requires docker compose.

EOF
    exit 1
}

function info {
    local text=$1
    echo -e "\033[0;36m $text \033[0m"
}

function warn {
    local text=$1
    echo -e "\033[0;31m $text \033[0m"
}

# The default gateway image that will be pulled and checked against in case
# no explicit gateway image tag is given.
DEFAULT_GW_TAG=v4.0.1

[[ -z $1 ]] && usage "$0"
export tag=$1
export gwtag=$2

if [[ -z $2 ]]; then
    warn "No explicit gateway tag given - Will use $DEFAULT_GW_TAG"
    gwtag=$DEFAULT_GW_TAG
fi

compose='docker-compose'
# use the compose client plugin if v2
[[ $(docker version --format='{{ .Client.Version }}') =~ 20.10 ]] && compose='docker compose'

#create the tmp directory to hold pump data.
TMPDIR=$(mktemp -d)
if [[ -e $TMPDIR ]]; then
    rm -f "$TMPDIR/*.csv"
else
    mkdir "$TMPDIR"
fi

trap cleanup EXIT

$compose up -d

GWBASE="http://localhost:8080"

curlf() {
    curl --header 'content-type:application/json' -s --show-error "$@"
}

cleanup() {
    $compose down
    warn "Cleaning up temporary dir $TMPDIR"
    rm -f "$TMPDIR/*.csv"
    rmdir "$TMPDIR"
}

# Add the test API - keyless APIs are not getting exported when pump is run.
info "Adding a test API to the Tyk GW..."
curlf --header "x-tyk-authorization: 352d20ee67be67f6340b4c0605b044b7" \
    -XPOST --data @data/api.authenabled.json ${GWBASE}/tyk/apis

# Add a corresponding key
info "Adding a key for the added API..."
KEY=$(curlf --header "x-tyk-authorization: 352d20ee67be67f6340b4c0605b044b7" -XPOST --data @data/key.json ${GWBASE}/tyk/keys | jq -r '.key')

# Hot reload gateway
info "Executing gateway hot reload..."
curlf --header "x-tyk-authorization: 352d20ee67be67f6340b4c0605b044b7" \
    ${GWBASE}/tyk/reload/group

# Wait a while for reload
sleep 2

# Get the key from the previous step and access the API endpoint with the key and a custom user agent.
info "Accessing the added API with a custom user agent string..."
curl -v --header "Authorization: $KEY" --header "User-Agent: HAL9000" \
    "${GWBASE}/test/"

# Sleep a while till the record gets exported
info "Waiting for the data to be exported...."
sleep 20

# Search for our custom  user agent in the csv file.
if grep "HAL9000" /tmp/pump-data/*.csv
then
    info "CSV Pump test completed successfully.."
else
    warn "CSV pump test failed.."
    exit 1
fi

exit 0
