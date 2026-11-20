#!/bin/zsh
# ci.sh <B> <blocks>: full macOS bench on a prepared work dir <B> holding apps/v1/Nevermind.app and
# apps/v2/Nevermind Next.app. Builds the observer, makes the fixture and a disposable keychain, measures the
# observer floor, runs one explore pass per version to read the row below "Settings", then the AB/BA blocks.
set -eu
B=$1; BLOCKS=$2; H=${0:A:h}
mkdir -p $B/harness $B/runs $B/explore
# Spotlight skips folders named *.noindex: without this it indexes every 51,000-file profile copy and loads the VM.
mkdir -p $B/profiles.noindex $B/fixture-master.noindex
ln -sfn $B/profiles.noindex $B/profiles; ln -sfn $B/fixture-master.noindex $B/fixture-master
cp $H/*.swift $H/*.sh $H/*.py $B/harness/
(cd $B/harness && swiftc -O -o nvmbench main.swift observer.swift session.swift calib.swift explore.swift titles.swift probe.swift diagnostics.swift)
python3 -I $B/harness/gen_fixture.py $B/fixture-master
DIGEST=$(python3 -I $B/harness/fixture_digest.py $B/fixture-master/Documents/bench)
echo "fixture: $DIGEST"
[[ $DIGEST == "51000 18133a583130f4acdde93f87ee9f275acc4e832393c15d44039894c4e252b6fe" ]] || { echo "fixture digest mismatch"; exit 3; }
export KCPW=$B/kcpw
$B/harness/make_keychain_template.sh $B $KCPW
$B/harness/nvmbench calib 60 $B/runs/observer-floor.csv 2>$B/runs/observer-floor.log || echo "observer floor calibration failed rc=$?"

explore() { # explore <v>: primed profile, then one explore launch
  local V=$1 ID=explore-$1
  $B/harness/prepare.sh $B $V $ID mem-setup 0 "Welcome to Nevermind" "Search actions" "Search Settings" "" >/dev/null
  $B/harness/run_one.sh $B session $B/runs/$ID/config.json; echo "$ID prime rc=$?"
  python3 -I - $B/runs/$ID/config.json <<'PY'
import json,sys
c=json.load(open(sys.argv[1])); c.update(memoryOnly=False, stopAfterIndex=False)
json.dump(c,open(sys.argv[1],"w"),indent=1)
PY
  security unlock-keychain -p "$(<$KCPW)" $B/profiles/$ID/home/Library/Keychains/bench.keychain-db
  $B/harness/run_one.sh $B explore $B/runs/$ID/config.json; echo "$ID explore rc=$?"
  python3 -I $B/harness/pick_second_row.py $B/runs/$ID/session.log >| $B/explore/second-row-$V.txt || { echo "STOP: explore $V found no second Settings row"; exit 8; }
  echo "$V row 2: $(<$B/explore/second-row-$V.txt)"
}
explore v2
explore v1
$B/harness/run_blocks.sh $B 1 $BLOCKS "$(<$B/explore/second-row-v1.txt)" "$(<$B/explore/second-row-v2.txt)"
