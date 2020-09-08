#!/bin/bash

set -exo pipefail

: ${SIGNKEY:="12B5D62C28F57592D1575BD51ED14C59E37DAC20"}
: ${BUILDPKGS:="1"}
: ${ARCH:=amd64}
: ${PKG_PREFIX:=tyk-pump}

if [ $BUILDPKGS == "1" ]; then
    echo Configuring gpg-agent-config to accept a passphrase
    mkdir ~/.gnupg && chmod 700 ~/.gnupg
    cat >> ~/.gnupg/gpg-agent.conf <<EOF
allow-preset-passphrase
debug-level expert
log-file /tmp/gpg-agent.log
EOF
    gpg-connect-agent reloadagent /bye

    echo "Importing signing key"
    gpg --list-keys | grep -w $SIGNKEY && echo "Key exists" || gpg --batch --import $BUILDTOOLSDIR/tyk.io.signing.key
    bash $BUILDTOOLSDIR/unlock-agent.sh $SIGNKEY
fi

bdir=build
echo "Creating build dir: $bdir"
mkdir -p $bdir

# ---- APP BUILD START ---
echo "Building application"
go build && mv tyk-pump $bdir
# ---- APP BUILD END ---

# ---- CREATE TARGET FOLDER ---
echo "Copying pump files"
cp -R install $bdir/
cp pump.example.conf $bdir/${PKG_PREFIX}.conf
cp LICENSE.md $bdir/
cp README.md $bdir/

echo "Making tarball"
tar -C $bdir -pczf ${PKG_PREFIX}-${ARCH}-${VERSION}.tar.gz .

FPMCOMMON=(
    --name tyk-pump
    --description "Tyk Pump to move analytics data from Redis to any supported back end"
    -v $VERSION
    --vendor "Tyk Technologies Ltd"
    -m "<info@tyk.io>"
    --url "https://tyk.io"
    -s dir
    -C $bdir
    --before-install $bdir/install/before_install.sh
    --after-install $bdir/install/post_install.sh
    --after-remove $bdir/install/post_remove.sh
    --config-files /opt/tyk-pump/pump.conf
)
FPMRPM=(
    --before-upgrade $bdir/install/post_remove.sh
    --after-upgrade $bdir/install/post_install.sh
)

if [ $BUILDPKGS == "1" ]; then
    echo "Building $ARCH packages"
    fpm "${FPMCOMMON[@]}" -a $ARCH -t deb --deb-user tyk --deb-group tyk ./=/opt/tyk-pump
    fpm "${FPMCOMMON[@]}" "${FPMRPM[@]}" -a $ARCH -t rpm --rpm-user tyk --rpm-group tyk  ./=/opt/tyk-pump

    echo "Signing $ARCH RPM"
    rpm --define "%_gpg_name Team Tyk (package signing) <team@tyk.io>" \
        --define "%__gpg /usr/bin/gpg" \
        --addsign *.rpm || (cat /tmp/gpg-agent.log; exit 1)
    echo "Signing $ARCH DEB"
    dpkg-sig --sign builder -k $SIGNKEY $i || (cat /tmp/gpg-agent.log; exit 1)
fi
