// Package loxberrylog macht LoxBerrys nativen, pro-Plugin einstellbaren
// "Log-Level"-Schalter (in der Plugin-Verwaltung, aktiviert durch
// CUSTOM_LOGLEVELS=true in plugin.cfg) auch fuer diesen Go-Daemon nutzbar.
//
// Real bestaetigt (2026-09-18, siehe Plan-Datei/Session-Notizen): dieser
// Schalter ist eingebaut ein REINES Perl/PHP-Feature - LoxBerry::Log.pm
// liest den gewaehlten Wert, kein Go/Node/Bash-Aequivalent existiert im
// LoxBerry-Core (auch nicht in den offiziellen Referenz-Plugins des
// LoxBerry-Erfinders selbst, z.B. Audioserver4Home/SmartFleet-Client -
// beide nutzen ausschliesslich Perl dafuer). Der gewaehlte Wert landet in
// der vom Core selbst verwalteten Datei
// $LBHOMEDIR/data/system/plugindatabase.json, als Feld "loglevel" (int,
// 0=Off, 3=Error, 4=Warning, 6=Info, 7=Debug) innerhalb des per Plugin-MD5
// verschluesselten Eintrags unter "plugins". Dieses Package liest genau
// diese Datei direkt selbst (undokumentiertes, aber einzig verfuegbares
// Verhalten fuer einen Nicht-Perl-Prozess) und pollt sie periodisch, damit
// eine Aenderung des Schalters ohne Neustart des Diensts wirkt - exakt das
// Verhalten, das LoxBerry::Log.pm fuer Perl-Skripte intern auch hat
// (dortiges Polling-Intervall ca. 60s, hier identisch uebernommen).
package loxberrylog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

const (
	LevelOff     = 0
	LevelError   = 3
	LevelWarning = 4
	LevelInfo    = 6
	LevelDebug   = 7

	// defaultLevel greift, solange plugindatabase.json (noch) nicht
	// gelesen werden konnte, sowie als dauerhafter Fallback, falls der
	// eigene Plugin-Eintrag darin nicht gefunden wird (z.B. lokaler
	// Testlauf ohne echte LoxBerry-Umgebung) - bewusst LevelError (3),
	// derselbe Default, den plugininstall.pl selbst fuer neu installierte
	// Plugins mit CUSTOM_LOGLEVELS=true setzt (siehe Kommentar oben).
	defaultLevel = LevelError
)

type pluginDBEntry struct {
	Folder           string `json:"folder"`
	LogLevel         *int   `json:"loglevel"`
	LogLevelsEnabled *int   `json:"loglevels_enabled"`
}

type pluginDatabase struct {
	Plugins map[string]pluginDBEntry `json:"plugins"`
}

// Watcher haelt das aktuell konfigurierte Log-Level fuer GENAU EIN Plugin
// (identifiziert ueber dessen Ordnernamen, siehe NewWatcher) im Speicher,
// atomar les-/schreibbar, damit Log-Aufrufe aus beliebigen Goroutinen ohne
// Lock pruefen koennen.
type Watcher struct {
	dbPath string
	folder string
	level  atomic.Int32
}

// NewWatcher erstellt den Watcher fuer den Plugin-Ordner "folder" (z.B.
// "easeemqtt"). dbPath ist der Pfad zu plugindatabase.json; leer bedeutet
// "$LBHOMEDIR/data/system/plugindatabase.json" mit $LBHOMEDIR-Fallback
// "/opt/loxberry" (identischer Fallback wie an anderer Stelle im Daemon,
// siehe internal/config).
func NewWatcher(folder, dbPath string) *Watcher {
	if dbPath == "" {
		home := os.Getenv("LBHOMEDIR")
		if home == "" {
			home = "/opt/loxberry"
		}
		dbPath = filepath.Join(home, "data", "system", "plugindatabase.json")
	}
	w := &Watcher{dbPath: dbPath, folder: folder}
	w.level.Store(defaultLevel)
	w.refresh()
	return w
}

// FolderFromConfigPath leitet den Plugin-Ordnernamen aus dem Config-Datei-
// Pfad ab (".../config/plugins/<folder>/config.json" -> "<folder>") -
// robuster als ihn hart zu verdrahten, falls LoxBerry bei einer Name/
// Folder-Kollision einen abweichenden Ordnernamen vergibt (siehe
// postroot.sh-Kommentare zu genau diesem Fall). Bei einem rein lokalen
// Testlauf (relativer Pfad wie "config.json") liefert das schlicht "."
// zurueck - der Watcher findet dann keinen passenden Eintrag und bleibt
// beim sicheren Default, siehe defaultLevel.
func FolderFromConfigPath(cfgPath string) string {
	return filepath.Base(filepath.Dir(cfgPath))
}

func (w *Watcher) refresh() {
	data, err := os.ReadFile(w.dbPath)
	if err != nil {
		return // Datei (noch) nicht da/lesbar - beim zuletzt bekannten Level bleiben
	}
	var db pluginDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return
	}
	for _, entry := range db.Plugins {
		if entry.Folder != w.folder {
			continue
		}
		// loglevels_enabled=0 (oder Feld fehlt) bedeutet: CUSTOM_LOGLEVELS
		// war beim Install nicht aktiv, oder der gespeicherte Wert ist der
		// PluginDB.pm-Sentinel "-1" (siehe Package-Kommentar) - in beiden
		// Faellen bleibt der sichere Default bestehen statt einen
		// unsinnigen/negativen Wert zu uebernehmen.
		if entry.LogLevelsEnabled != nil && *entry.LogLevelsEnabled == 0 {
			return
		}
		if entry.LogLevel != nil && *entry.LogLevel >= LevelOff && *entry.LogLevel <= LevelDebug {
			w.level.Store(int32(*entry.LogLevel))
		}
		return
	}
}

// Start pollt plugindatabase.json im Hintergrund, bis ctxDone geschlossen
// wird - gleiches ca.-60s-Intervall wie LoxBerry::Log.pm intern (siehe
// Package-Kommentar), damit eine Aenderung des Dropdowns ohne Neustart des
// Diensts spuerbar wird.
func (w *Watcher) Start(ctxDone <-chan struct{}) {
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctxDone:
				return
			case <-ticker.C:
				w.refresh()
			}
		}
	}()
}

// Level liefert das aktuell konfigurierte Level (0/3/4/6/7, siehe
// Level*-Konstanten).
func (w *Watcher) Level() int {
	return int(w.level.Load())
}

// Enabled prueft, ob eine Meldung der Dringlichkeit "level" (Level*-
// Konstante) beim aktuell konfigurierten Level ausgegeben werden soll -
// gleiche Syslog-aehnliche Semantik wie LoxBerry selbst: eine Meldung wird
// gezeigt, wenn das konfigurierte Level >= ihrer eigenen Dringlichkeit ist.
func (w *Watcher) Enabled(level int) bool {
	return w.Level() >= level
}
