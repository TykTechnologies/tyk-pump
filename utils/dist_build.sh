#!/bin/bash
: ${ORGDIR:="/src/github.com/TykTechnologies"}
: ${SIGNKEY:="729EA673"}
: ${BUILDPKGS:="1"}
TYK_PUMP_SRC_DIR=$ORGDIR/tyk-pump
BUILDTOOLSDIR=$TYK_PUMP_SRC_DIR/build_tools

echo "Set version number"
: ${VERSION:=$(perl -n -e'/v(\d+).(\d+).(\d+)/'' && print "$1\.$2\.$3"' version.go)}

if [ $BUILDPKGS == "1" ]; then
    echo "Importing signing key"
    gpg --list-keys | grep -w $SIGNKEY && echo "Key exists" || gpg --import $BUILDTOOLSDIR/build_key.key
fi

DESCRIPTION="Tyk Pump to move analytics data from Redis to any supported back end"
RELEASE_DIR="$TYK_PUMP_SRC_DIR/build"
BUILD="tyk-pump-$VERSION"
BUILD_DIR="$RELEASE_DIR/$BUILD"

cd $TYK_PUMP_SRC_DIR

echo "Creating build folder ($BUILD_DIR)"
mkdir -p $BUILD_DIR

# ---- APP BUILD START ---
echo "Building application"
gox -osarch="linux/arm64 linux/amd64 linux/386"
# ---- APP BUILD END ---

# ---- CREATE TARGET FOLDER ---
echo "Copying pump files"
cd $TYK_PUMP_SRC_DIR
cp -R install $BUILD_DIR/
cp pump.example.conf $BUILD_DIR/pump.conf
cp LICENSE.md $BUILD_DIR/
cp README.md $BUILD_DIR/

cd $RELEASE_DIR
echo "Removing old builds"
rm -f *.deb
rm -f *.rpm
rm -f *.tar.gz

echo "LINUX"
FPMCOMMON=(
    --name tyk-pump
    --description "$DESCRIPTION"
    -v $VERSION
    --vendor "Tyk Technologies Ltd"
    -m "<info@tyk.io>"
    --url "https://tyk.io"
    -s dir
    -C $BUILD_DIR
    --before-install $BUILD_DIR/install/before_install.sh
    --after-install $BUILD_DIR/install/post_install.sh
    --after-remove $BUILD_DIR/install/post_remove.sh
    --config-files /opt/tyk-pump/pump.conf
)
FPMRPM=(
    --before-upgrade $BUILD_DIR/install/post_remove.sh
    --after-upgrade $BUILD_DIR/install/post_install.sh
)

for arch in i386 amd64 arm64
do
    echo "Creating $arch Tarball"
    cd $TYK_PUMP_SRC_DIR
    mv tyk-pump_linux_${arch/i386/386} $BUILD_DIR/tyk-pump
    cd $RELEASE_DIR
    tar -pczf $RELEASE_DIR/tyk-pump-$arch-$VERSION.tar.gz $BUILD/

    if [ $BUILDPKGS == "1" ]; then
        echo "Building $arch packages"
        fpm "${FPMCOMMON[@]}" -a $arch -t deb --deb-user tyk --deb-group tyk ./=/opt/tyk-pump
        fpm "${FPMCOMMON[@]}" "${FPMRPM[@]}" -a $arch -t rpm --rpm-user tyk --rpm-group tyk  ./=/opt/tyk-pump

        rpmName="tyk-pump-$VERSION-1.${arch/amd64/x86_64}.rpm"
        echo "Signing $arch RPM"
        $BUILDTOOLSDIR/rpm-sign.exp $rpmName
    fi
done
