#!/bin/zsh
# prepare.sh <B> <version v1|v2> <id> <mode explore|session> <launch> [rootText actionText settingsText]
set -eu
KCPW=${KCPW:?path to the disposable keychain password file}
B=$1; V=$2; ID=$3; MODE=$4; L=$5
P=$B/profiles/$ID; S=${NB_SHORT:-/tmp/nb}/$ID; O=$B/runs/$ID
mkdir -p $P/home/.config/nevermind $O $S/xdg $S/tmp; chmod 700 $S $S/xdg $S/tmp
cp -c -R $B/fixture-master/Documents $P/home/
mkdir -p $P/home/Library/Keychains
cp $B/keychain-template/bench.keychain-db $P/home/Library/Keychains/bench.keychain-db
ln -s bench.keychain-db $P/home/Library/Keychains/login.keychain-db
security unlock-keychain -p "$(<$KCPW)" $P/home/Library/Keychains/bench.keychain-db
printf '%s\n' '{"version":1,"settings":{"paletteHotkey":"Alt+Shift+Space"}}' >| $P/home/.config/nevermind/config.json
if [[ $V == v1 ]]; then
  APP="$B/apps/v1/Nevermind.app"; EXE="$APP/Contents/MacOS/Nevermind"; mkdir -p $P/userdata; ARGS="[\"--user-data-dir=$P/userdata\"]"
else
  APP="$B/apps/v2/Nevermind Next.app"; EXE="$APP/Contents/MacOS/Nevermind Next"; ARGS="[]"
fi
codesign --verify --strict --deep "$APP" >| $O/codesign-verify.txt 2>&1
shasum -a 256 "$EXE" >| $O/exe-sha256.txt
python3 -I $B/harness/fixture_digest.py $P/home/Documents/bench >| $O/fixture-digest.txt
python3 -I - "$O/config.json" <<PY
import json,sys
json.dump({"version":"$V","launch":$L,"exe":"$EXE","args":$ARGS,"home":"$P/home","shortDir":"$S","outDir":"$O",
 "hotkey":"alt+shift+space","actionKey":"cmd+k","warmups":1,"trials":3,
 "rootText":"${6:-}","actionText":"${7:-}","settingsQuery":"settings","settingsText":"${8:-}","searchSecondRow":"${9:-}",
 "timeoutS":5,"calibrateOnly":False,"stopAfterIndex":"$MODE"=="setup","memoryOnly":"$MODE".startswith("mem"),
 "memoryPhaseOrder":["startup","settle","hidden-idle"] if "$MODE"=="mem-measure" else ["first-run"],
 "memoryPhases":{"startup":20,"settle":30,"hidden-idle":30,"first-run":60}},open(sys.argv[1],"w"),indent=1)
PY
echo $O/config.json
