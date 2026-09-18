#!/bin/bash
systemctl stop easeemqtt.service 2>/dev/null
systemctl disable easeemqtt.service 2>/dev/null
rm -f /etc/systemd/system/easeemqtt.service
rm -f /etc/logrotate.d/easeemqtt

systemctl daemon-reload
exit 0
