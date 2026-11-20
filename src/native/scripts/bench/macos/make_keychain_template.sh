#!/bin/zsh
# Creates one empty disposable keychain, then restores the user's keychain search list exactly.
set -u
B=$1; PWFILE=$2
T=$B/keychain-template/bench.keychain-db
mkdir -p $B/keychain-template $B/isolation
security list-keychains -d user | sed 's/^ *"//; s/"$//' >| $B/isolation/keychain-searchlist-before.txt
security default-keychain -d user | sed 's/^ *"//; s/"$//' >| $B/isolation/keychain-default-before.txt
restore() {
  ORIG=("${(@f)$(cat $B/isolation/keychain-searchlist-before.txt)}")
  security list-keychains -d user -s "${ORIG[@]}"
  security list-keychains -d user | sed 's/^ *"//; s/"$//' >| $B/isolation/keychain-searchlist-after.txt
  security default-keychain -d user | sed 's/^ *"//; s/"$//' >| $B/isolation/keychain-default-after.txt
  if cmp -s $B/isolation/keychain-searchlist-before.txt $B/isolation/keychain-searchlist-after.txt && cmp -s $B/isolation/keychain-default-before.txt $B/isolation/keychain-default-after.txt; then
    echo "search list and default keychain restored exactly"
  else
    echo "MISMATCH after restore"; exit 9
  fi
}
trap restore EXIT
umask 077
PW=$(openssl rand -hex 24)
print -rn -- "$PW" >| $PWFILE
security create-keychain -p "$PW" $T
security set-keychain-settings $T
security lock-keychain $T
security unlock-keychain -p "$(<$PWFILE)" $T && echo "template unlock verified"
