#!/bin/bash
# preupgrade.sh laeuft bei einem UPDATE (nicht bei der Erstinstallation) als
# User "loxberry", BEVOR LoxBerrys eigener Installer die vorherige Version
# entfernt ("purge_installation" in sbin/plugininstall.pl - verifiziert
# gegen ein echtes Install-Protokoll vom 2026-09-08: das loescht bei JEDEM
# Update komplett "config/plugins/easeemqtt/", inklusive config.json UND
# secret.key, nicht nur bei einer Deinstallation, wie zunaechst angenommen).
# Sichert beide Dateien an einen Ort ausserhalb von config/plugins/... -
# postroot.sh (das NACH der Neuanlage des Ordners laeuft) stellt sie wieder
# her. Siehe dort fuer die Gegenseite dieses Mechanismus.
#
# Echte Parameterreihenfolge (siehe plugininstall.pl, identisch zu
# postroot.sh): $1=tempfile $2=pname $3=pfolder $4=pversion $5=lbhomedir
# $6=tempfolder
LBHOMEDIR="${5:-/opt/loxberry}"
PFOLDER="${3:-easeemqtt}"
CFGDIR="$LBHOMEDIR/config/plugins/$PFOLDER"
BACKUP_DIR="/tmp/easeemqtt-preupgrade-backup"

mkdir -p "$BACKUP_DIR"
[ -f "$CFGDIR/config.json" ] && cp -a "$CFGDIR/config.json" "$BACKUP_DIR/config.json"
[ -f "$CFGDIR/secret.key" ] && cp -a "$CFGDIR/secret.key" "$BACKUP_DIR/secret.key"

exit 0
