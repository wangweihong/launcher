#!/bin/bash

#
# set localtime
ln -sf /usr/share/zoneinfo/$LOCALTIME /etc/localtime

function replace_vars() {
  eval "cat <<EOF
  $(<$2)
EOF
  " > $1
}

replace_vars '/etc/keepalived/keepalived.conf' '/etc/keepalived/10_keepalived.conf'
replace_vars '/etc/keepalived/notify.sh' '/etc/keepalived/10_notify.sh'
replace_vars '/etc/keepalived/health.sh' '/etc/keepalived/10_health.sh'
chmod +rx /etc/keepalived/notify.sh /etc/keepalived/health.sh

# Run
/usr/sbin/keepalived -P -C -d -D -S 7 -f /etc/keepalived/keepalived.conf --dont-fork --log-console
