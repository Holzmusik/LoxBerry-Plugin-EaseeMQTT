# LoxBerry-Plugin-EaseeMQTT (intern: easeemqtt)

Reine Protokollbruecke Easee-Wallboxen (beliebig viele, nicht fest auf eine
Anzahl begrenzt) &lt;-&gt; Loxone ueber MQTT. Lastmanagement/Entscheidungslogik
bleibt **komplett in Loxone** - dieser Dienst uebersetzt nur 1:1 zwischen
Easees Cloud-API und MQTT, ohne eigene Logik.

Struktur/Konventionen bewusst 1:1 vom Schwester-Plugin
[LoxBerry-Plugin-KNXtoLOX](../LoxBerry-Plugin-KNXtoLOX) uebernommen (dort
real-hardware-verifiziert): `plugin.cfg`-INI-Format, `postroot.sh`-Muster,
Perl/CGI-Single-Page-App mit identischem Design (`.em-app` statt `.kx-app`),
eigenes `applog()` statt `LoxBerry::Log`, `LBHOMEDIR`-Direktpfad fuer
`general.json`.

## Architektur

```
Easee Cloud (SignalR-Push + REST) <-> easeemqtt (Go, systemd: easeemqtt.service) <-> MQTT-Broker <-> Loxone (native MQTT)
```

Anders als bei KNXtoLOX ist hier **nur ein einziger Dienst** noetig: Loxone
spricht MQTT nativ (Miniserver Gen2) und verbindet sich direkt mit dem
Broker - keine zusaetzliche MQTT&lt;-&gt;Loxone-Websocket-Bruecke.

Der eigene Go-Daemon (`daemon/`, aus dem mitgelieferten Quellcode gebaut,
kein externer `git clone` noetig):

1. Login + proaktiver Token-Refresh (5 Min vor Ablauf der 1h-Gueltigkeit,
   zusaetzlich Retry-mit-Refresh bei HTTP 401)
2. SignalR-Verbindung zu `https://streams.easee.com/hubs/chargers`,
   Auto-Reconnect mit moderatem Backoff (siehe Sicherheitshinweis unten),
   Abonnement ALLER in `config.json` gelisteten Charger-IDs
3. Uebersetzt empfangene Observation-Events -&gt; retained MQTT-State-Topics
4. Abonniert MQTT-Kommando-Topics -&gt; echte Easee-REST-Aufrufe (keine eigene
   Entscheidungslogik)

## MQTT-Topic-Schema

Praefix konfigurierbar (Default `easee/`). Ziel (Nutzer-Vorgabe 2026-09-07):
**alle** Wallbox-Informationen landen in MQTT, und die Wallboxen sind
**komplett steuerbar** - Lastmanagement/Entscheidungslogik bleibt trotzdem
extern (Loxone), der Daemon uebersetzt nur.

**Status (Daemon -&gt; Loxone, retained)** - `<prefix><charger_id>/state/<name>`,
für **alle 170 bekannten Easee-Observation-IDs** (generiert aus der echten
`ObservationID`-Enum in `evcc-io/evcc`s Produktionscode, siehe
`daemon/internal/bridge/bridge.go`'s `observationTopics`-Tabelle für die
vollständige, verbindliche Liste). Die alltagsrelevantesten:
```
<prefix><charger_id>/state/opmode                    Lademodus (numerischer Easee-Code)
<prefix><charger_id>/state/pilot_mode                 IEC-61851-Ladezustand (A-F)
<prefix><charger_id>/state/power_kw                    aktuelle Leistung
<prefix><charger_id>/state/session_energy_kwh          Energie dieser Ladesession
<prefix><charger_id>/state/lifetime_energy_kwh         Energie gesamt (Lebenszeit)
<prefix><charger_id>/state/cable_locked                Kabel verriegelt (0/1)
<prefix><charger_id>/state/is_enabled                  Charger aktiviert (0/1)
<prefix><charger_id>/state/dynamic_current_a            aktuell erlaubtes Limit (fluechtig)
<prefix><charger_id>/state/max_current_a                 dauerhaftes Maximal-Limit
<prefix><charger_id>/state/output_phase                  1- oder 3-phasig aktiv
<prefix><charger_id>/state/derating_active / derated_current   thermische Drosselung
<prefix><charger_id>/state/error_code / fatal_error_code / error_string
<prefix><charger_id>/state/connected_to_cloud             Cloud-Verbindung der Box selbst
<prefix><charger_id>/state/current_t2 .. current_t5       Phasenstroeme (Rohbenennung, siehe Hinweis unten)
<prefix><charger_id>/state/in_volt_t1_t2 .. in_volt_t4_t5 Netzspannungen zwischen den Terminals (Rohbenennung)
<prefix><charger_id>/state/temp_max                       Temperaturueberwachung
... (alle weiteren 150+ IDs ebenso, siehe observationTopics in bridge.go)
<prefix>bridge/status   (online/offline, Last-Will des Daemons selbst)
```

**Kommandos (Loxone -&gt; Daemon)** - komplettes Kommando-Set, alle Pfade/
Feldnamen gegen den echten `pyeasee`-Sourcecode verifiziert:
```
<prefix><charger_id>/cmd/start|stop|pause|resume|toggle|reboot|override_schedule   (beliebiger Payload loest aus)
<prefix><charger_id>/set/dynamic_current       Zahl in A - fluechtiges Limit, auf P1/P2/P3 symmetrisch angewandt
<prefix><charger_id>/set/max_current           Ganzzahl in A - dauerhaftes Limit
<prefix><charger_id>/set/enabled               0/1 - Charger komplett aktivieren/deaktivieren
<prefix><charger_id>/set/cable_lock            0/1 - Kabel dauerhaft verriegeln
<prefix><charger_id>/set/single_phase_limit    0/1 - auf 1-phasiges Laden begrenzen
<prefix><charger_id>/set/phase_mode            1=1-phasig fest, 2=Auto, 3=3-phasig fest
<prefix><charger_id>/set/smart_charging        0/1
<prefix><charger_id>/set/led_brightness        0-100
<prefix><charger_id>/set/idle_current          0/1 - Reststrom nach Ladeende signalisieren
```
Bool-Payloads akzeptieren grosszuegig `0/1`, `true/false` und `on/off`
(siehe `parseBool()` in `bridge.go`) - unabhaengig davon, welcher
Loxone-MQTT-Output-Baustein verwendet wird.

**Hinweis `current_t2..t5`/`in_volt_t*_t*`**: die exakte Zuordnung
Easee-Terminal-Nummer -&gt; Netzphase (L1/L2/L3) wurde in der
Entwicklungssession nicht gegen echte Hardware verifiziert - deshalb bewusst
Easees eigene Rohbenennung uebernommen statt eine moeglicherweise falsche
L1/L2/L3-Zuordnung vorzutaeuschen. Bei Bedarf anhand eines echten Mitschnitts
waehrend eines laufenden Ladevorgangs korrekt ummappen.

## Konfiguration (`config.json`)

Eine einzige Datei, sowohl von der Web-UI (`api.cgi`) geschrieben als auch
vom Go-Daemon direkt gelesen (kein YAML-Split wie bei KNXtoLOX noetig -
JSON::PP/encoding-json sind ein robustes, symmetrisches Paar):

```json
{
  "easee": { "username": "", "password": "", "chargers": [{"id": "", "name": ""}] },
  "mqtt": {
    "use_local_broker": true,
    "host": "", "port": 1883, "username": "", "password": "",
    "topic_prefix": "easee/", "client_id": "easeemqtt",
    "enabled_observations": [31, 47, 48, 100, 103, 109, 110, 115, 116, 119, 120, 121, 124, 150, 182, 183, 184, 185, 190, 191, 192, 193, 194, 195, 196, 197, 198, 199, 219, 250, 251]
  }
}
```

`use_local_broker=true`: `api.cgi` loest Host/Port/Zugangsdaten bei jedem
Speichern frisch aus LoxBerrys eigenem `general.json` auf und schreibt die
AUFGELOESTEN Werte in `config.json` - der Go-Daemon selbst kennt LoxBerry/
`general.json` nicht, bleibt dadurch als reine Protokollbruecke einfach.

`enabled_observations`: Positivliste der Observation-IDs, die als MQTT-
State-Topics publiziert werden (Kommando-Topics sind davon unberuehrt, die
sind immer alle aktiv) - konfigurierbar in der Web-UI ("Daten"-Seite,
Checkbox-Liste mit vorausgewaehltem Basis-Setup), siehe unten. Fehlt der
Schluessel komplett (z.B. eine sehr alte `config.json`), publiziert der
Daemon sicherheitshalber ALLES statt versehentlich nichts.

## Web-UI

Single-Page-App (`webfrontend/htmlauth/index.cgi` + `api.cgi`), Design 1:1 an
KNXtoLOX/MiraiPanel angelehnt. Seiten: Uebersicht (Dienst-Status,
vollstaendige Kommando-Tabelle mit Payload-Format, echte Topic-Beispiele mit
den konfigurierten Prefix/Charger-IDs zum Copy-Paste), Konto &amp; Wallboxen (Login-Test, "Ladegeraete
suchen"-Picker gegen die echte Easee-API, editierbare Charger-Tabelle -
beliebig viele Eintraege), MQTT (lokaler/externer Broker), **Daten**
(Checkbox-Liste aller 170 Observation-IDs nach Kategorie gruppiert, mit
Suche und "Basis-Setup"/"Alle"/"Keine"-Schnellauswahl - steuert, welche
Werte als MQTT-State-Topics landen), **Loxone-Import** (generiert fertige
Copy-Paste-Werte fuer LoxBerrys "MQTT Gateway"-Plugin - Befehlserkennungs-
Strings fuer Virtuelle UDP-Eingaenge bzw. Namen fuer Virtuelle Eingaenge bei
HTTP, plus `publish`-Befehle fuer alle Kommando-Topics als Virtueller-
Ausgang-Vorlage; Format verifiziert gegen die echten LoxBerry-Wiki-
Anleitungen, siehe unten), Status (Dienst-Status, Rohlog), Debug
(Web-UI-eigenes Log, Passwoerter/Tokens werden vor dem Loggen redigiert).

### Loxone-Integration ueber LoxBerrys "MQTT Gateway"-Plugin

Diese Bridge verbindet sich selbst mit MQTT - Loxone braucht dafuer ein
separates, bereits existierendes LoxBerry-Plugin ("MQTT Gateway"), das
zwischen MQTT-Broker und Miniserver vermittelt. Ablauf (Details:
[MQTT -&gt; Loxone](https://wiki.loxberry.de/konfiguration/widget_help/widget_mqtt/mqtt_gateway/mqtt_schritt_fur_schritt_mqtt_loxone),
[Loxone -&gt; MQTT](https://wiki.loxberry.de/konfiguration/widget_help/widget_mqtt/mqtt_gateway/mqtt_schritt_fur_schritt_loxone_mqtt)):

1. Im MQTT-Gateway-Plugin (Tab **Abonnements**) die gewuenschten Easee-Topics
   einmalig abonnieren (kein API-Zugriff dafuer vorhanden - manueller Schritt,
   die "Loxone-Import"-Seite kann nur die DANACH benoetigten Werte liefern).
2. Auf der "Loxone-Import"-Seite Protokoll waehlen (UDP empfohlen - ein
   einziger Virtueller UDP-Eingang fuer beliebig viele Werte, statt bei HTTP
   pro Wert ein eigener Virtueller Eingang samt eigener Berechtigungen).
3. Generierte Werte in Loxone Config einfuegen:
   - **State-Topics (UDP)**: je Zeile ein *Virtueller UDP Eingang Befehl*
     unter einem gemeinsamen *Virtuellen UDP Eingang* (Port = Gateway UDP
     In-Port, Standard 11884) - "Befehlserkennung" = generierter
     `MQTT:\i<topic>=\i\v`-String.
   - **State-Topics (HTTP)**: je Zeile ein *Virtueller Eingang*, dessen
     "Bezeichnung" EXAKT dem generierten Namen entsprechen muss (Topic mit
     `/` -&gt; `_`) - "Als Digitaleingang verwenden" nicht aktivieren.
   - **Kommandos**: ein gemeinsamer *Virtueller Ausgang* (Adresse
     `/dev/udp/<loxberry>/<Gateway-UDP-In-Port>`), darunter je Zeile ein
     *Virtueller Ausgang Befehl* mit den generierten `publish <topic>
     <wert>`-Strings als "Befehl bei EIN"/"Befehl bei AUS".

Unsere flachen Einzelwert-Topics (ein Wert pro Topic, kein gebuendeltes JSON)
passen dem Gateway-Muster besonders gut - die im Wiki beschriebene
JSON-Aufteilung ("Schritt 3") entfaellt komplett.

## Installation

1. Plugin als ZIP ueber "Plugin installieren" hochladen.
2. `postroot.sh` baut `easeemqtt` aus dem mitgelieferten `daemon/`-Quellcode
   (`go mod tidy && go build`, braucht Internetzugriff zum Go-Modul-Proxy),
   richtet den systemd-Dienst ein.
3. Unter "Konto &amp; Wallboxen": Easee-Zugangsdaten eintragen, testen,
   Ladegeraete suchen und der Tabelle hinzufuegen.
4. Unter "MQTT": lokalen Broker bestaetigen oder externen Broker eintragen.
5. Speichern - der Dienst startet automatisch mit der neuen Konfiguration neu.

## Bekannte offene Punkte

- **Real-Hardware-Status (Stand 2026-09-07)**: Installation (Go-Build,
  systemd) UND Laufzeit (Login, SignalR-Verbindung, Observation-Events,
  MQTT-Publish) sind bestaetigt funktionsfaehig - erster echter End-to-End-
  Nachweis, kein reiner Struktur-Check mehr.
- **Gefixt (2026-09-07, per echtem MQTT-Mitschnitt gefunden)**: die
  Charger-ID im SignalR-Observation-JSON heisst `Mid`/`mid`, nicht
  `chargerId` wie urspruenglich geraten - dadurch blieb `ChargerID` immer
  leer und alle konfigurierten Wallboxen kollidierten auf demselben
  MQTT-Topic-Ast (fehlende `<charger_id>` im Pfad), statt eigene Topics zu
  bekommen. Verifiziert gegen den echten `evcc-io/evcc`-Sourcecode (dessen
  `Observation`-Struct hat gar keine JSON-Tags - Go matcht ohne Tag
  case-insensitiv gegen den Go-Feldnamen `Mid`, `id`/`dataType`/`value`
  trafen davor nur zufaellig per Gross-/Kleinschreibungs-Fallback). Braucht
  ein Neu-Bauen des Daemons (Plugin neu installieren), um zu greifen.
- **Bestaetigt korrekt** (aus demselben Mitschnitt): die Observation-ID-Tabelle
  in `bridge.go` (109 opmode, 120 power_kw, 121 session_energy_kwh, 124
  lifetime_energy_kwh, 103 cable_locked, 47 max_current_a, 48
  dynamic_current_a, 119 error_code, 219 fatal_error_code) UND der
  `SubscribeWithCurrentState`-Aufruf funktionieren wie erwartet - beide
  Punkte waren vorher als unverifiziert geflaggt.
- `current_t2..t5` -&gt; L1/L2/L3-Zuordnung weiterhin offen - im Mitschnitt
  standen alle vier bei 0 (kein Ladevorgang aktiv), eine echte Zuordnung
  braucht einen Mitschnitt waehrend eines laufenden Ladevorgangs.
- Easee-Passwort liegt (wie bei KNXtoLOX fuer KNX/MQTT-Zugangsdaten) im
  Klartext in `config.json`.
- Der `streams.easee.com`-SignalR-Endpunkt wurde in einem oeffentlichen
  GitHub-Thread (Mai 2026) im Zusammenhang mit einem IP-Ratelimit
  ("Blackhole") genannt - der Daemon verwendet deshalb bewusst moderates
  Backoff statt aggressiver Reconnect-Versuche.
- Kein `go build`/`go vet` auf dem Entwicklungsrechner moeglich (kein Go dort
  installiert) - Aenderungen am Daemon werden weiterhin nur strukturell
  geprueft, bis der naechste `postroot.sh`-Build auf echter Hardware sie
  bestaetigt (dort inzwischen mehrfach erfolgreich gelaufen, siehe oben).
