#!/bin/sh

# Super hacky release script

# ----- SET THE VERSION NUMBER -----
CURRENTVERS=$(perl -n -e'/v(\d+).(\d+).(\d+).(\d+)/'' && print "v$1\.$2\.$3\.$4"' version.go)

echo "Current version is: " $CURRENTVERS

echo -n "Major version [ENTER]: "
read maj 
echo -n "Minor version [ENTER]: "
read min 
echo -n "Patch version [ENTER]: "
read patch 
echo -n "Release version [ENTER]: "
read rel 

NEWVERSION="v$maj.$min.$patch.$rel"
NEWVERSION_DHMAKE="$maj.$min.$patch.$rel"
echo "Setting new version in source: " $NEWVERSION

perl -pi -e 's/var VERSION string = \"(.*)\"/var VERSION string = \"'$NEWVERSION'\"/g' version.go

# ----- END VERSION SETTING -----

# Clear build folder:
echo "Clearing build folder..."
rm -rf ~/go/src/github.com/lonelycode/tyk-pump/build/*

VERSION=$NEWVERSION_DHMAKE
SOURCEBIN=tyk-pump
SOURCEBINPATH=~/go/src/github.com/lonelycode/tyk-pump
i386BINDIR=$SOURCEBINPATH/build/i386/tyk-pump.linux.i386-$VERSION
amd64BINDIR=$SOURCEBINPATH/build/amd64/tyk-pump.linux.amd64-$VERSION
armBINDIR=$SOURCEBINPATH/build/arm/tyk-pump.linux.arm-$VERSION

i386TGZDIR=$SOURCEBINPATH/build/i386/tgz/tyk-pump.linux.i386-$VERSION
amd64TGZDIR=$SOURCEBINPATH/build/amd64/tgz/tyk-pump.linux.amd64-$VERSION
armTGZDIR=$SOURCEBINPATH/build/arm/tgz/tyk-pump.linux.arm-$VERSION

cd $SOURCEBINPATH

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

echo "Creating Deb Package for AMD64"
cd $amd64TGZDIR/
fpm -n tyk-pump -v $VERSION  --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a amd64 -s dir -t deb ./=/opt/tyk-pump
fpm -n tyk-pump -v $VERSION  --rpm-sign  --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a amd64 -s dir -t rpm ./=/opt/tyk-pump

package_cloud yank tyk/tyk-pump/ubuntu/precise *.deb
package_cloud yank tyk/tyk-pump/ubuntu/trusty *.deb
package_cloud yank tyk/tyk-pump/debian/jessie *.deb
package_cloud yank tyk/tyk-pump/el/6 *.rpm
package_cloud yank tyk/tyk-pump/el/7 *.rpm

package_cloud push tyk/tyk-pump/ubuntu/precise *.deb
package_cloud push tyk/tyk-pump/ubuntu/trusty *.deb
package_cloud push tyk/tyk-pump/debian/jessie *.deb
package_cloud push tyk/tyk-pump/el/6 *.rpm
package_cloud push tyk/tyk-pump/el/7 *.rpm


# echo "Creating Deb Package for i386"
cd $i386TGZDIR/
fpm -n tyk-pump -v $VERSION --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a i386 -s dir -t deb ./=/opt/tyk-pump
fpm -n tyk-pump -v $VERSION --rpm-sign --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a i386 -s dir -t rpm ./=/opt/tyk-pump

package_cloud yank tyk/tyk-pump/ubuntu/precise *.deb
package_cloud yank tyk/tyk-pump/ubuntu/trusty *.deb
package_cloud yank tyk/tyk-pump/debian/jessie *.deb
package_cloud yank tyk/tyk-pump/el/6 *.rpm
package_cloud yank tyk/tyk-pump/el/7 *.rpm

package_cloud push tyk/tyk-pump/ubuntu/precise *.deb
package_cloud push tyk/tyk-pump/ubuntu/trusty *.deb
package_cloud push tyk/tyk-pump/debian/jessie *.deb
package_cloud push tyk/tyk-pump/el/6 *.rpm
package_cloud push tyk/tyk-pump/el/7 *.rpm

echo "Creating Deb Package for ARM"
cd $armTGZDIR/
fpm -n tyk-pump -v $VERSION --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a arm -s dir -t deb ./=/opt/tyk-pump
fpm -n tyk-pump -v $VERSION --rpm-sign --after-install $amd64TGZDIR/install/post_install.sh --after-remove $amd64TGZDIR/install/post_remove.sh -a arm -s dir -t rpm ./=/opt/tyk-pump

package_cloud yank tyk/tyk-pump/ubuntu/precise *.deb
package_cloud yank tyk/tyk-pump/ubuntu/trusty *.deb
package_cloud yank tyk/tyk-pump/debian/jessie *.deb
package_cloud yank tyk/tyk-pump/el/6 *.rpm
package_cloud yank tyk/tyk-pump/el/7 *.rpm

package_cloud push tyk/tyk-pump/ubuntu/precise *.deb
package_cloud push tyk/tyk-pump/ubuntu/trusty *.deb
package_cloud push tyk/tyk-pump/debian/jessie *.deb
package_cloud push tyk/tyk-pump/el/6 *.rpm
package_cloud push tyk/tyk-pump/el/7 *.rpm

#echo "Re-installing"
#cd $amd64TGZDIR/
#sudo dpkg -i tyk-gateway_1.9.0.0_amd64.deb