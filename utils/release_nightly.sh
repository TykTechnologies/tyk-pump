#!/bin/sh
set -e
# Super hacky release script

# ----- SET THE VERSION NUMBER -----
CURRENTVERS=$(perl -n -e'/v(\d+).(\d+).(\d+).(\d+)/'' && print "v$1\.$2\.$3\.$4"' version.go)

DATE=$(date +'%m-%d-%Y')
BUILDVERS="$CURRENTVERS-nightly-$DATE" 
echo "Build will be: " $BUILDVERS

NEWVERSION=$BUILDVERS
echo "Setting new version in source: " $NEWVERSION

perl -pi -e 's/var VERSION string = \"(.*)\"/var VERSION string = \"'$NEWVERSION'\"/g' version.go

# ----- END VERSION SETTING -----

# Clear build folder:
echo "Clearing build folder..."
rm -rf ~/go/src/github.com/lonelycode/tyk-pump/build/*

VERSION=$NEWVERSION_DHMAKE
SOURCEBIN=tyk-pump
SOURCEBINPATH=~/tyk-pump
i386BINDIR=$SOURCEBINPATH/build/i386/tyk-pump.linux.i386-$VERSION
amd64BINDIR=$SOURCEBINPATH/build/amd64/tyk-pump.linux.amd64-$VERSION
armBINDIR=$SOURCEBINPATH/build/arm/tyk-pump.linux.arm-$VERSION

i386TGZDIR=$SOURCEBINPATH/build/i386/tgz/tyk-pump.linux.i386-$VERSION
amd64TGZDIR=$SOURCEBINPATH/build/amd64/tgz/tyk-pump.linux.amd64-$VERSION
armTGZDIR=$SOURCEBINPATH/build/arm/tgz/tyk-pump.linux.arm-$VERSION

cd $SOURCEBINPATH

echo "Getting deps"
go get -t -d -v ./...

echo "Fixing MGO Version"
cd $GOPATH/src/gopkg.in/mgo.v2/
git checkout tags/r2016.02.04
cd $SOURCEBINPATH

echo "Installing cross-compiler"
go get github.com/mitchellh/gox

echo "Creating TGZ dirs"
mkdir -p $i386TGZDIR
mkdir -p $amd64TGZDIR
mkdir -p $armTGZDIR


echo "Building binaries"
gox -os="linux"

rc=$?
if [[ $rc != 0 ]] ; then
    echo "Something went wrong with the build, please fix and retry"
    rm -rf rm -rf /home/tyk/tyk/build/*
    exit $rc
fi

mkdir $i386TGZDIR/install

cp -R $SOURCEBINPATH/install/* $i386TGZDIR/install
cp $SOURCEBINPATH/pump.example.conf $i386TGZDIR/pump.conf

cp -R $i386TGZDIR/* $amd64TGZDIR
cp -R $i386TGZDIR/* $armTGZDIR

cp tyk-pump_linux_386 $i386TGZDIR/$SOURCEBIN
cp tyk-pump_linux_amd64 $amd64TGZDIR/$SOURCEBIN
cp tyk-pump_linux_arm $armTGZDIR/$SOURCEBIN

echo "Compressing"
cd $i386TGZDIR/../
tar -pczf $i386TGZDIR/../tyk-pump-linux-i386-$VERSION.tar.gz tyk-pump.linux.i386-$VERSION/

cd $amd64TGZDIR/../
tar -pczf $amd64TGZDIR/../tyk-pump-linux-amd64-$VERSION.tar.gz tyk-pump.linux.amd64-$VERSION/

cd $armTGZDIR/../
tar -pczf $armTGZDIR/../tyk-pump-linux-arm-$VERSION.tar.gz tyk-pump.linux.arm-$VERSION/

echo "Creating release directory and copying files"
cd $SOURCEBINPATH
RELEASEPATH=$SOURCEBINPATH/build/release
mkdir $RELEASEPATH
cp $i386TGZDIR/../*.tar.gz $RELEASEPATH
cp $amd64TGZDIR/../*.tar.gz $RELEASEPATH
cp $armTGZDIR/../*.tar.gz $RELEASEPATH
cp utils/index.html $RELEASEPATH
echo "Done"