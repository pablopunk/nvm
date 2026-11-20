#!/bin/zsh
# run_one.sh <B> <nvmbench mode> <config.json>: runs the harness, then stops any surviving test process tree by exact PID.
B=$1; MODE=$2; CFG=$3; O=${CFG:h}
$B/harness/nvmbench $MODE $CFG 2>>$O/harness-stderr.log; RC=$?
if [[ -s $O/app.pid ]]; then
  APP=$(<$O/app.pid)
  KIDS=($(pgrep -P $APP))
  if kill -0 $APP 2>/dev/null; then echo "cleanup: test pid $APP still alive after harness rc=$RC" >>$O/harness-stderr.log; kill $APP; sleep 3; kill -9 $APP $KIDS 2>/dev/null; fi
  for k in $KIDS; do kill -0 $k 2>/dev/null && kill -9 $k; done
fi
exit $RC
