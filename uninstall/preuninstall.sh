#!/bin/bash
systemctl stop easeemqtt.service 2>/dev/null
systemctl disable easeemqtt.service 2>/dev/null
rm -f /etc/systemd/system/easeemqtt.service

systemctl daemon-reload
exit 0
