#!/bin/zsh
# run_blocks.sh <A> <first> <last> <v1SecondRow> <v2SecondRow>: AB/BA blocks; per block and version a fresh profile,
# an unmeasured 60 s first-run launch (no keys), then one measured session (1 warmup + 3 trials per action).
set -u
A=$1; FIRST=$2; LAST=$3; R1=$4; R2=$5
export KCPW=${KCPW:?}
idle_s() { ioreg -c IOHIDSystem | awk '/HIDIdleTime/ {print int($NF/1e9); exit}'; }
ctx() { { date -u +%FT%TZ; sysctl -n vm.loadavg; sysctl -n vm.swapusage; pmset -g therm | grep -v '^Note'; memory_pressure | tail -1
  ps -Ao pcpu=,comm= -r | head -5 | awk '{n=$2; sub(/.*\//,"",n); print $1, n}'; } >| $1; }
for k in $(seq $FIRST $LAST); do
  if (( k % 2 )); then ORDER=(v1 v2); else ORDER=(v2 v1); fi
  for V in $ORDER; do
    ID=$V-b$(printf %02d $k); [[ $V == v1 ]] && SEC=$R1 || SEC=$R2
    $A/harness/prepare.sh $A $V $ID mem-setup $k "Welcome to Nevermind" "Search actions" "Search Settings" "$SEC" >/dev/null || { echo "STOP prepare failed $ID"; exit 5; }
    ctx $A/runs/$ID/host-context.txt
    $A/harness/run_one.sh $A session $A/runs/$ID/config.json; echo "$ID prime rc=$?"
    I=$(idle_s); echo "$ID idle after keyless prime: ${I}s"
    if [[ ${BENCH_IDLE_GUARD:-1} == 1 ]] && (( I < 50 )); then echo "STOP: HID input during the keyless prime (idle ${I}s): someone may be using the Mac"; exit 6; fi
    M=$A/runs/$ID-run; mkdir -p $M
    python3 -I - $A/runs/$ID/config.json $M <<'PY'
import json,sys
c=json.load(open(sys.argv[1])); c.update(outDir=sys.argv[2], memoryOnly=False, stopAfterIndex=False)
json.dump(c,open(sys.argv[2]+"/config.json","w"),indent=1)
PY
    security unlock-keychain -p "$(<$KCPW)" $A/profiles/$ID/home/Library/Keychains/bench.keychain-db
    ctx $M/host-context.txt
    $A/harness/run_one.sh $A session $M/config.json; RC=$?; echo "$ID run rc=$RC"
    /bin/rm -rf $A/profiles/$ID/home/Documents
    (( RC == 4 )) && { echo "STOP: screen locked or display asleep"; exit 4; }
    grep -q aborted-unsafe $M/trials.jsonl && { echo "STOP: unsafe dialog seen in $ID"; exit 7; }
  done
done
echo "ALL BLOCKS DONE"
