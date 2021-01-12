#!/bin/sh

#
# Use this utility to import libgit2 and git2go sources into
# vendor directory.
#

set -e

DIR="$(pwd)"
GIT2GO_PATH="$DIR/pkgdir/github.com/libgit2/git2go"
LIBGIT2_PATH="$GIT2GO_PATH/vendor/libgit2"
GIT2GO_VERSION="v28.8.6"


if [ -d $GIT2GO_PATH ]; then
  rm -rf $GIT2GO_PATH
fi

git clone https://github.com/libgit2/git2go.git $GIT2GO_PATH

cd $GIT2GO_PATH
git checkout $GIT2GO_VERSION
git submodule update --init
make install-static

cd $DIR
rm -rf $GIT2GO_PATH/.git
rm -rf $LIBGIT2_PATH/.git
