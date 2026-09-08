#!/bin/sh
# Build (or extend) the apt repository that GitHub Pages serves.
#
#   build.sh <incoming> <site> <version>
#
# <incoming> holds the .deb files of one release, <site> is the checkout of
# the gh-pages branch — empty on the first run — and <version> names the
# commit. Packages already in <site> stay, so an older release remains
# installable and `apt upgrade` has somewhere to come from. The signing key
# must already be in the gpg keyring; it is the only secret key there.
#
# Layout, the one apt expects:
#   pool/main/h/homa/<deb>                     the packages
#   dists/stable/main/binary-<arch>/Packages   the index per architecture
#   dists/stable/Release, InRelease, Release.gpg
#   homa.gpg, homa.asc                         the public key, binary and armored
set -eu

incoming=$1
site=$2
version=$3

mkdir -p "$site/pool/main/h/homa"
cp "$incoming"/*.deb "$site/pool/main/h/homa/"

cd "$site"

for arch in amd64 arm64; do
	mkdir -p "dists/stable/main/binary-$arch"
	dpkg-scanpackages --arch "$arch" pool/main > "dists/stable/main/binary-$arch/Packages"
	gzip -9 -k -f "dists/stable/main/binary-$arch/Packages"
done

apt-ftparchive \
	-o APT::FTPArchive::Release::Origin=homa \
	-o APT::FTPArchive::Release::Label=homa \
	-o APT::FTPArchive::Release::Suite=stable \
	-o APT::FTPArchive::Release::Codename=stable \
	-o APT::FTPArchive::Release::Components=main \
	-o "APT::FTPArchive::Release::Architectures=amd64 arm64" \
	-o "APT::FTPArchive::Release::Description=homa, peer-to-peer terminal chat" \
	release dists/stable > dists/stable/Release

gpg --batch --yes --armor --detach-sign --output dists/stable/Release.gpg dists/stable/Release
gpg --batch --yes --clearsign --output dists/stable/InRelease dists/stable/Release
gpg --batch --yes --export > homa.gpg
gpg --batch --yes --armor --export > homa.asc

# Pages must not run the tree through Jekyll, which drops dotfiles and dirs.
touch .nojekyll

echo "apt repository built for $version:"
find pool -name '*.deb' | sort
