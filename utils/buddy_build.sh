#!/bin/bash
echo Set version number
export VERSION=$(perl -n -e'/v(\d+).(\d+).(\d+).(\d+)/'' && print "v$1\.$2\.$3\.$4"' version.go)

echo Generating key
[[ $(gpg --list-keys | grep -w 729EA673) ]] && echo "Key exists" || gpg --import build_key.key

# Clear build folder:
echo "Clearing build folder..."
mkdir -p build/
rm -rf build/*

SOURCEBIN=tyk-pump
SOURCEBINPATH=/src/github.com/TykTechnologies/tyk-pump
i386BINDIR=$SOURCEBINPATH/build/i386/tyk-pump.linux.i386-$VERSION
amd64BINDIR=$SOURCEBINPATH/build/amd64/tyk-pump.linux.amd64-$VERSION
armBINDIR=$SOURCEBINPATH/build/arm/tyk-pump.linux.arm64-$VERSION

i386TGZDIR=$SOURCEBINPATH/build/i386/tgz/tyk-pump.linux.i386-$VERSION
amd64TGZDIR=$SOURCEBINPATH/build/amd64/tgz/tyk-pump.linux.amd64-$VERSION
armTGZDIR=$SOURCEBINPATH/build/arm/tgz/tyk-pump.linux.arm64-$VERSION
export PACKAGECLOUDREPO=tyk-pump-auto

cd $SOURCEBINPATH

echo "Creating TGZ dirs"
mkdir -p $i386TGZDIR
mkdir -p $amd64TGZDIR
mkdir -p $armTGZDIR


echo "Building binaries"
gox -osarch="linux/arm64 linux/amd64 linux/386"

rc=$?
if [[ $rc != 0 ]] ; then
    echo "Something went wrong with the build, please fix and retry"
    rm -rf build/*
    exit $rc
fi

mkdir $i386TGZDIR/install

cp -R $SOURCEBINPATH/install/* $i386TGZDIR/install
cp $SOURCEBINPATH/pump.example.conf $i386TGZDIR/pump.conf

cp -R $i386TGZDIR/* $amd64TGZDIR
cp -R $i386TGZDIR/* $armTGZDIR

cp tyk-pump_linux_386 $i386TGZDIR/$SOURCEBIN
cp tyk-pump_linux_amd64 $amd64TGZDIR/$SOURCEBIN
cp tyk-pump_linux_arm64 $armTGZDIR/$SOURCEBIN

echo "Compressing"
cd $i386TGZDIR/../
tar -pczf $i386TGZDIR/../tyk-pump-linux-i386-$VERSION.tar.gz tyk-pump.linux.i386-$VERSION/

cd $amd64TGZDIR/../
tar -pczf $amd64TGZDIR/../tyk-pump-linux-amd64-$VERSION.tar.gz tyk-pump.linux.amd64-$VERSION/

cd $armTGZDIR/../
tar -pczf $armTGZDIR/../tyk-pump-linux-arm-$VERSION.tar.gz tyk-pump.linux.arm-$VERSION/

echo "Creating Deb Package for AMD64"
cd $amd64TGZDIR/
fpm -n tyk-pump -v $VERSION  --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a amd64 -s dir -t deb ./=/opt/tyk-pump
fpm -n tyk-pump -v $VERSION  --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a amd64 -s dir -t rpm ./=/opt/tyk-pump

AMDDEBNAME="tyk-pump_"$VERSION"_amd64.deb"
AMDRPMNAME="tyk-pump-"$VERSION"-1.x86_64.rpm"

echo "Signing AMD RPM"
~/build_tools/rpm-sign.exp $amd64TGZDIR/$AMDRPMNAME

package_cloud push tyk/$PACKAGECLOUDREPO/ubuntu/precise $AMDDEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/ubuntu/trusty $AMDDEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/debian/jessie $AMDDEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/el/6 $AMDRPMNAME
package_cloud push tyk/$PACKAGECLOUDREPO/el/7 $AMDRPMNAME


# echo "Creating Deb Package for i386"
cd $i386TGZDIR/
fpm -n tyk-pump -v $VERSION --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a i386 -s dir -t deb ./=/opt/tyk-pump
fpm -n tyk-pump -v $VERSION --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a i386 -s dir -t rpm ./=/opt/tyk-pump

i386DEBNAME="tyk-pump_"$VERSION"_i386.deb"
i386RPMNAME="tyk-pump-"$VERSION"-1.i386.rpm"

echo "Signing i386 RPM"
~/build_tools/rpm-sign.exp $i386TGZDIR/$i386RPMNAME

package_cloud push tyk/$PACKAGECLOUDREPO/ubuntu/precise $i386DEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/ubuntu/trusty $i386DEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/debian/jessie $i386DEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/el/6 $i386RPMNAME
package_cloud push tyk/$PACKAGECLOUDREPO/el/7 $i386RPMNAME

echo "Creating Deb Package for ARM"
cd $armTGZDIR/
fpm -n tyk-pump -v $VERSION --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a arm64 -s dir -t deb ./=/opt/tyk-pump
fpm -n tyk-pump -v $VERSION --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a arm64 -s dir -t rpm ./=/opt/tyk-pump

ARMDEBNAME="tyk-pump_"$VERSION"_arm64.deb"
ARMRPMNAME="tyk-pump-"$VERSION"-1.arm64.rpm"

echo "Signing Arm RPM"
~/build_tools/rpm-sign.exp $armTGZDIR/$ARMRPMNAME

package_cloud push tyk/$PACKAGECLOUDREPO/ubuntu/precise $ARMDEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/ubuntu/trusty $ARMDEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/debian/jessie $ARMDEBNAME
package_cloud push tyk/$PACKAGECLOUDREPO/el/6 $ARMRPMNAME
package_cloud push tyk/$PACKAGECLOUDREPO/el/7 $ARMRPMNAME