#!/usr/bin/perl
# EaseeMQTT Haupt-UI - eine einzige Single-Page-App. Optik bewusst 1:1 vom
# Schwester-Plugin KNXtoLOX übernommen (Klasse .kx-app -> .em-app, sonst
# unverändert), das wiederum an MiraiPanel angelehnt ist - alle drei Plugins
# sollen gleich aussehen/sich gleich bedienen, siehe Plan-Datei.
use LoxBerry::System;
use LoxBerry::Web;
use CGI;
use strict;
use warnings;

my $cgi = CGI->new;
print $cgi->header(-charset => 'utf-8');
LoxBerry::Web::lbheader("Easee-MQTT Bridge");
print <<'HTML';
<style>
.em-app {
  --bg:      var(--lb-bg, #f7f7f7);
  --surface: var(--lb-card-bg, #fff);
  --surf2:   var(--lb-input-bg, #f5f5f5);
  --brd:     var(--lb-border-color, #e5e5e5);
  --txt:     var(--lb-text, #171717);
  --txt2:    var(--lb-text-muted, #737373);
  --acc:     var(--lb-primary, #6dac20);
  --warn:    var(--lb-warning, #ca8a04);
  --danger:  var(--lb-danger, #dc2626);
  --radius:  var(--lb-radius, 12px);
  color: var(--txt); font: 14px/1.5 system-ui, sans-serif;
}
.em-app, .em-app * { box-sizing: border-box; margin: 0; padding: 0; }

.em-app .shell { display: flex; min-height: 640px; background: var(--bg); border-radius: var(--radius); overflow: hidden; border: 1px solid var(--brd); }
.em-app .sidebar { width: 220px; background: var(--surface); border-right: 1px solid var(--brd); display: flex; flex-direction: column; flex-shrink: 0; }
.em-app .sidebar-logo { padding: 20px 18px 12px; font-size: 17px; font-weight: 700; color: var(--acc); letter-spacing: -.3px; border-bottom: 1px solid var(--brd); }
.em-app .sidebar-logo span { color: var(--txt2); font-weight: 400; font-size: 12px; display: block; margin-top: 2px; }
.em-app .nav { flex: 1; padding: 8px 0; }
.em-app .nav-item { display: flex; align-items: center; gap: 10px; padding: 10px 18px; cursor: pointer; color: var(--txt2); border-left: 3px solid transparent; transition: all .15s; }
.em-app .nav-item:hover { color: var(--txt); background: var(--surf2); }
.em-app .nav-item.active { color: var(--acc); border-left-color: var(--acc); background: rgba(102,205,0,.07); }
.em-app .main { flex: 1; overflow: hidden; padding: 28px; min-height: 0; display: flex; flex-direction: column; }

.em-app .page { display: none; }
.em-app .page.active { display: flex; flex-direction: column; flex: 1; min-height: 0; overflow-y: auto; }
.em-app h2 { font-size: 18px; font-weight: 600; margin-bottom: 20px; }
.em-app h3 { font-size: 14px; font-weight: 600; margin-bottom: 14px; color: var(--txt2); text-transform: uppercase; letter-spacing: .5px; }
.em-app .card { background: var(--surface); border: 1px solid var(--brd); border-radius: var(--radius); padding: 20px; margin-bottom: 16px; }
.em-app .card-title { font-size: 15px; font-weight: 600; margin-bottom: 16px; }
.em-app .card-title-row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.em-app .card-title-row .card-title { margin-bottom: 0; }
.em-app .card-toggle { cursor: pointer; color: var(--txt2); font-size: 12px; user-select: none; }
.em-app .card-toggle:hover { color: var(--txt); }
.em-app .log-hidden { display: none; }
.em-app .hint { font-size: 12px; color: var(--txt2); margin-top: 6px; }

.em-app select {
  background-color: var(--surf2) !important;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%23888' d='M6 8L1 3h10z'/%3E%3C/svg%3E");
  background-repeat: no-repeat; background-position: right 12px center;
  appearance: none; -webkit-appearance: none; -moz-appearance: none;
  border: 1px solid var(--brd) !important; border-radius: 8px; padding: 9px 32px 9px 12px;
  color: var(--txt) !important; font-size: 14px; outline: none; cursor: pointer;
}
.em-app select:focus { border-color: var(--acc) !important; box-shadow: 0 0 0 3px rgba(102,205,0,.15); }
.em-app select option { background: var(--surf2) !important; color: var(--txt) !important; }
.em-app input[type=text], .em-app input[type=number], .em-app input[type=password] {
  background: var(--surf2); border: 1px solid var(--brd); border-radius: 8px; padding: 9px 12px; color: var(--txt); font-size: 14px; outline: none;
}
.em-app input:focus { border-color: var(--acc); }
/* Eigener div-basierter Toggle statt <input type=checkbox> - siehe KNXtoLOX
   für die Begründung (jQuery Mobile "verbessert" native Checkboxen trotz
   data-enhance="false"/data-role="none" nachweislich weiterhin). */
.em-app .toggle {
  width: 36px; height: 20px; min-width: 36px; flex-shrink: 0;
  border-radius: 10px; background: var(--surf2); border: 1px solid var(--brd);
  position: relative; cursor: pointer; transition: background .15s, border-color .15s;
}
.em-app .toggle::after {
  content: ''; position: absolute; top: 1px; left: 1px;
  width: 16px; height: 16px; border-radius: 50%; background: var(--txt2);
  transition: transform .15s, background .15s;
}
.em-app .toggle[data-checked="true"] { background: var(--acc); border-color: var(--acc); }
.em-app .toggle[data-checked="true"]::after { transform: translateX(16px); background: #000; }
.em-app .checkfield label { flex: 1; cursor: pointer; }

.em-app .field { margin-bottom: 14px; }
.em-app .field label { display: block; font-size: 12px; color: var(--txt2); margin-bottom: 5px; }
.em-app .field input, .em-app .field select { width: 100%; }
.em-app .row { display: flex; gap: 12px; }
.em-app .row .field { flex: 1; }
.em-app .checkfield { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }

.em-app .btn { display: inline-flex; align-items: center; gap: 7px; padding: 9px 16px; border-radius: 8px; border: none; cursor: pointer; font-size: 14px; font-weight: 500; transition: opacity .15s; }
.em-app .btn:hover { opacity: .85; }
.em-app .btn-primary { background: var(--acc); color: #000; }
.em-app .btn-secondary { background: var(--surf2); color: var(--txt); border: 1px solid var(--brd); }
.em-app .btn-danger { background: var(--danger); color: #fff; }
.em-app .btn-sm { padding: 6px 12px; font-size: 13px; }
.em-app .btn-group { display: flex; gap: 10px; margin-top: 18px; margin-bottom: 20px; flex-wrap: wrap; }

.em-app table.ga-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.em-app table.ga-table th { text-align: left; color: var(--txt2); font-weight: 600; padding: 6px 8px; border-bottom: 1px solid var(--brd); font-size: 11px; text-transform: uppercase; letter-spacing: .3px; position: sticky; top: 0; background: var(--surface); z-index: 1; }
.em-app table.ga-table td { padding: 5px 8px; border-bottom: 1px solid var(--brd); color: var(--txt) !important; }
.em-app table.ga-table input, .em-app table.ga-table select { width: 100%; padding: 6px 8px; font-size: 13px; }
.em-app table.ga-table .ga-del { cursor: pointer; color: var(--danger); font-size: 16px; }
.em-app table.ga-table td.mono { font-variant-numeric: tabular-nums; color: var(--txt2) !important; white-space: nowrap; }
.em-app .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }

.em-app .badge { font-size: 11px; padding: 3px 8px; border-radius: 20px; background: var(--surf2); color: var(--txt2); border: 1px solid var(--brd); }
.em-app .status-bar { display: flex; align-items: center; gap: 8px; padding: 10px 14px; background: var(--surf2); border-radius: 8px; margin-bottom: 16px; font-size: 13px; }
.em-app .status-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--txt2); flex-shrink: 0; }
.em-app .status-dot.ok { background: var(--acc); }
.em-app .status-dot.err { background: var(--danger); }
.em-app pre.log { background: #111; color: #0f0; padding: 12px; border-radius: 8px; overflow: auto; max-height: 45vh; font-size: 12px; }
.em-app .toast { position: fixed; bottom: 24px; right: 24px; background: var(--surf2); border: 1px solid var(--brd); border-radius: 10px; padding: 12px 18px; font-size: 13px; box-shadow: 0 4px 20px rgba(0,0,0,.5); transform: translateY(80px); opacity: 0; transition: all .25s; z-index: 999; }
.em-app .toast.show { transform: translateY(0); opacity: 1; }
.em-app .toast.ok  { border-color: var(--acc); }
.em-app .toast.err { border-color: var(--danger); }
.em-app .divider { border: none; border-top: 1px solid var(--brd); margin: 20px 0; }

/* Ladegeräte-Picker-Modal (Easee-Konto -> gefundene Charger auswählen) */
.em-app .modal-backdrop { display: none; position: fixed; inset: 0; background: rgba(0,0,0,.75); z-index: 1000; align-items: center; justify-content: center; }
.em-app .modal-backdrop.show { display: flex; }
/* Fest verdrahtete Farben statt var(--surface)/var(--txt)+!important: der
   erste Versuch (2026-09-07) änderte nichts, weil !important zwar die
   Kaskade gewinnt, aber NICHT ändert, worauf die Variable selbst auflöst -
   wenn LoxBerrys eigenes --lb-card-bg leicht transparent oder --lb-text zu
   nah an --lb-card-bg ist, benutzt "background: var(--surface) !important"
   am Ende trotzdem exakt denselben (durchscheinenden/kontrastarmen) Wert.
   Deshalb hier bewusst KEINE Variable mehr für Hintergrund/Text im Modal -
   garantiert deckende, garantiert kontrastreiche Literalfarben, unabhängig
   davon was LoxBerrys Theme für --lb-card-bg/--lb-text tatsächlich liefert
   (gleiches Prinzip wie pre.log weiter oben, das aus genau diesem Grund auch
   schon feste Farben statt Variablen nutzt). */
.em-app .modal {
  background: #1c1c20 !important;
  box-shadow: 0 12px 48px rgba(0,0,0,.7);
  border: 1px solid #3a3a40; border-radius: var(--radius); width: 480px; max-width: 92vw; max-height: 80vh; display: flex; flex-direction: column; padding: 20px;
}
.em-app .modal .card-title { color: #f2f2f2 !important; }
.em-app .modal-list { overflow-y: auto; flex: 1; margin-top: 10px; display: flex; flex-direction: column; gap: 2px; }
.em-app .modal-item { padding: 8px 10px; border-radius: 6px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; }
.em-app .modal-item:hover { background: #2a2a30; }
.em-app .modal-item .name { font-weight: 500; color: #f2f2f2 !important; }
.em-app .modal-item .meta { font-size: 11px; color: #9a9aa2 !important; }

/* Observation-Checkbox-Liste (Daten-Seite) - eigener, kompakterer div-Toggle
   statt der grossen .toggle-Pille (bei 170 Zeilen zu viel vertikaler Platz)
   und bewusst KEIN natives <input type=checkbox> (jQuery Mobile "verbessert"
   das nachweislich trotz data-role="none", siehe .toggle-Kommentar oben). */
.em-app .obs-group { background: var(--surf2); border: 1px solid var(--brd); border-radius: 10px; padding: 12px 14px 14px; margin-top: 14px; }
.em-app .obs-group-title { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; font-size: 12px; font-weight: 600; color: var(--txt2); text-transform: uppercase; letter-spacing: .5px; margin-bottom: 10px; }
.em-app .obs-group-title .obs-group-count { text-transform: none; letter-spacing: normal; font-weight: 400; font-size: 11px; white-space: nowrap; }
.em-app .obs-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 2px 12px; }
.em-app .obs-row { display: flex; align-items: center; gap: 8px; padding: 5px 4px; border-radius: 6px; cursor: pointer; font-size: 13px; }
.em-app .obs-row:hover { background: var(--surface); }
.em-app .obs-check { width: 16px; height: 16px; min-width: 16px; border-radius: 4px; border: 1px solid var(--brd); background: var(--surface); position: relative; flex-shrink: 0; }
.em-app .obs-check[data-checked="true"] { background: var(--acc); border-color: var(--acc); }
.em-app .obs-check[data-checked="true"]::after { content: ''; position: absolute; left: 5px; top: 2px; width: 3px; height: 7px; border: solid #000; border-width: 0 2px 2px 0; transform: rotate(45deg); }
.em-app .obs-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.em-app .obs-id { color: var(--txt2); font-variant-numeric: tabular-nums; font-size: 11px; flex-shrink: 0; }

@media (max-width: 680px) {
  .em-app .shell { flex-direction: column; min-height: 0; overflow: visible; border-radius: 0; border: none; }
  .em-app .sidebar { width: 100%; flex-direction: row; flex-shrink: 0; border-right: none; border-bottom: 1px solid var(--brd); }
  .em-app .sidebar-logo { display: none; }
  .em-app .nav { flex-direction: row; padding: 0; overflow-x: auto; }
  .em-app .nav-item { flex-direction: column; gap: 3px; padding: 10px 14px; font-size: 11px; white-space: nowrap; border-left: none; border-bottom: 3px solid transparent; min-height: 52px; justify-content: center; align-items: center; }
  .em-app .nav-item.active { border-left-color: transparent; border-bottom-color: var(--acc); }
  .em-app .main { padding: 16px; overflow-y: visible; display: block; }
  .em-app .row { flex-direction: column; gap: 0; }
  .em-app .toast { left: 12px; right: 12px; bottom: 12px; }
}
</style>

<div class="em-app" data-enhance="false">
<div class="shell">

<aside class="sidebar">
  <div class="sidebar-logo">EaseeMQTT<span>Easee &lt;-&gt; Loxone Brücke</span></div>
  <nav class="nav">
    <div class="nav-item active" data-page="overview">Übersicht</div>
    <div class="nav-item" data-page="account">Konto &amp; Wallboxen</div>
    <div class="nav-item" data-page="mqtt">MQTT</div>
    <div class="nav-item" data-page="data">Daten</div>
    <div class="nav-item" data-page="loxone">Loxone-Import</div>
    <div class="nav-item" data-page="status">Status</div>
    <div class="nav-item" data-page="debug">Debug</div>
  </nav>
</aside>

<main class="main">

  <section id="page-overview" class="page active">
    <h2>Übersicht</h2>
    <div class="card">
      <div class="card-title">Dienst</div>
      <div class="status-bar"><span class="status-dot" id="dot-svc"></span><span id="txt-svc">easeemqtt (Easee &lt;-&gt; MQTT) ...</span></div>
      <div class="hint">
        Protokollbrücke: Easee-Cloud &lt;-&gt; MQTT.
      </div>
    </div>
    <div class="card">
      <div class="card-title">Konfigurierte Wallboxen</div>
      <div id="charger-summary" class="hint">lädt ...</div>
    </div>
  </section>

  <section id="page-account" class="page">
    <h2>Konto &amp; Wallboxen</h2>
    <div class="card">
      <div class="card-title">Easee-Konto</div>
      <div class="row">
        <div class="field"><label>Benutzername (E-Mail/Telefon mit Landesvorwahl)</label><input type="text" id="easee_username" data-role="none"></div>
        <div class="field"><label>Passwort</label><input type="password" id="easee_password" data-role="none"></div>
      </div>
      <div class="btn-group">
        <button class="btn btn-primary" onclick="testEasee()">Verbindung testen</button>
        <button class="btn btn-secondary" onclick="searchChargers()">Ladegeräte neu laden</button>
      </div>
      <span class="hint" id="easee-test-status"></span>
      <div class="hint">Nach erfolgreichem Login werden die im Easee-Konto vorhandenen Ladegeräte automatisch angezeigt - "Ladegeräte neu laden" nur nötig, falls seither ein neuer Charger im Konto dazugekommen ist.</div>
    </div>

    <div class="card">
      <div class="card-title">Wallboxen</div>
      <div class="hint" style="margin-bottom:10px">
        Aktive Wallboxen - über "Ladegeräte suchen" automatisch
        befüllen oder Charger-ID von Hand eintragen/ergänzen.
      </div>
      <div style="overflow-x:auto">
        <table class="ga-table">
          <thead><tr><th>Charger-ID</th><th>Bezeichnung</th><th></th></tr></thead>
          <tbody id="charger-tbody"></tbody>
        </table>
      </div>
      <div class="btn-group">
        <button class="btn btn-secondary btn-sm" onclick="addChargerRow()">+ Zeile</button>
        <button class="btn btn-primary" onclick="saveConfig()">Speichern</button>
      </div>
    </div>
  </section>

  <section id="page-mqtt" class="page">
    <h2>MQTT</h2>
    <div class="card">
      <div class="checkfield"><div class="toggle" id="use_local_broker" onclick="toggleSwitch('use_local_broker')"></div><label onclick="toggleSwitch('use_local_broker')">Lokalen LoxBerry-Broker verwenden</label></div>
      <div class="status-bar" id="local-broker-bar"><span class="status-dot ok"></span><span id="local-broker-info">MQTT-Broker (aus LoxBerry) ...</span></div>
      <div class="hint">Bei aktivem Schalter werden Host/Port/Zugangsdaten bei jedem Speichern automatisch aus LoxBerrys eigener MQTT-Gateway-Konfiguration übernommen.</div>
      <div class="divider"></div>
      <div id="external-broker-fields">
        <div class="row">
          <div class="field"><label>Broker-Host</label><input type="text" id="mqtt_host" data-role="none" placeholder="192.168.1.10"></div>
          <div class="field"><label>Port</label><input type="number" id="mqtt_port" data-role="none" placeholder="1883"></div>
        </div>
        <div class="row">
          <div class="field"><label>Benutzername</label><input type="text" id="mqtt_username" data-role="none"></div>
          <div class="field"><label>Passwort</label><input type="password" id="mqtt_password" data-role="none"></div>
        </div>
      </div>
      <div class="row">
        <div class="field"><label>Topic-Prefix</label><input type="text" id="topic_prefix" data-role="none" placeholder="easee/"></div>
        <div class="field"><label>MQTT-Client-ID</label><input type="text" id="client_id" data-role="none" placeholder="easeemqtt"></div>
      </div>
    </div>
    <div class="btn-group">
      <button class="btn btn-primary" onclick="saveConfig()">Speichern</button>
    </div>
  </section>

  <section id="page-data" class="page">
    <h2>Daten</h2>
    <div class="card">
      <div class="card-title">Welche Werte landen in MQTT?</div>
      <div class="hint" style="margin-bottom:14px">
        Easee liefert 170 verschiedene Werte pro Wallbox (von Ladeleistung
        bis WLAN-Diagnose) - hier auswählen, welche davon tatsächlich als
        MQTT-Topics publiziert werden. Lastmanagement/Steuerung bleibt davon
        unberührt (Kommando-Topics sind immer alle aktiv).
      </div>
      <div class="row" style="align-items:flex-end">
        <div class="field" style="flex:2">
          <label>Suche</label>
          <input type="text" id="obs-search" data-role="none" placeholder="z.B. current, temp, error ..." oninput="renderObsList()">
        </div>
        <div class="field"><span id="obs-count" class="hint" style="margin:0"></span></div>
      </div>
      <div class="btn-group" style="margin-top:0">
        <button class="btn btn-secondary btn-sm" onclick="setObsSelection('basic')">Basis-Setup</button>
        <button class="btn btn-secondary btn-sm" onclick="setObsSelection('all')">Alle auswählen</button>
        <button class="btn btn-secondary btn-sm" onclick="setObsSelection('none')">keine</button>
      </div>
      <div id="obs-groups"></div>
      <div class="btn-group">
        <button class="btn btn-primary" onclick="saveConfig()">Speichern</button>
      </div>
    </div>
  </section>

  <section id="page-loxone" class="page">
    <h2>Loxone-Import</h2>
    <div class="card">
      <div class="card-title">Wie kommen die Topics nach Loxone?</div>
      <div class="hint">
        Diese Bridge verbindet sich mit MQTT - Loxone selbst braucht dafür
        LoxBerrys eigenes <strong>"MQTT Gateway"</strong> als Vermittler. Dort müssen die
        Topics einmalig im Tab <strong>Abonnements</strong> abonniert werden (Wildcard auf Topic Präfix, oder jedes Topic separat).
        Sind die Topics abonniert, generiert diese Seite die fertigen Werte zum
        Copy-Paste für Loxone Config (Quelle: <a href="https://wiki.loxberry.de/konfiguration/widget_help/widget_mqtt/mqtt_gateway/mqtt_schritt_fur_schritt_mqtt_loxone" target="_blank" rel="noopener">LoxBerry-Wiki</a>).
      </div>
      <div class="hint">
        Bis zu 170 verschiedene Status-Werte pro Wallbox sind möglich -
        welche davon tatsächlich publiziert werden (und damit unten in der
        Tabelle auftauchen), wählst du auf der <strong>"Daten"</strong>-Seite
        aus. <code>&lt;prefix&gt;bridge/status</code> (online/offline) ist der
        Last-Will-Status des Dienstes selbst und erscheint deshalb nicht in
        der Tabelle - falls gewünscht, dafür einen eigenen Virtuellen
        Eingang/UDP-Befehl nach demselben Muster von Hand anlegen.
      </div>
      <div class="divider"></div>
      <div class="row">
        <div class="field" style="max-width:320px">
          <label>Protokoll im MQTT-Gateway (Tab "Gateway")</label>
          <select id="loxone-protocol" data-role="none" onchange="renderLoxoneExport()">
            <option value="udp">UDP (empfohlen - ein Eingang für alle Werte)</option>
            <option value="http">HTTP (ein Virtueller Eingang pro Wert)</option>
          </select>
        </div>
        <div class="field" style="max-width:320px">
          <label>Wallbox</label>
          <select id="loxone-charger" data-role="none" onchange="renderLoxoneExport()">
            <option value="">Alle Wallboxen</option>
          </select>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-title">Status-Topics -&gt; Virtuelle Eingänge</div>
      <div class="hint" id="loxone-state-hint" style="margin-bottom:10px"></div>
      <div class="hint" style="margin-bottom:10px">Pro Zeile ein Loxone-Objekt anlegen. Klick in ein Feld wählt dessen Inhalt komplett aus (Strg+C) - Bezeichnung und Befehl sind bewusst getrennte Felder, damit du nur genau das kopierst, was gerade gebraucht wird.</div>
      <div style="overflow-x:auto; max-height:420px; overflow-y:auto">
        <table class="ga-table" id="loxone-state-table">
          <thead><tr id="loxone-state-thead"></tr></thead>
          <tbody id="loxone-state-tbody"></tbody>
        </table>
      </div>
    </div>

    <div class="card">
      <div class="card-title">Kommandos -&gt; Virtueller Ausgang + Befehle</div>
      <div class="hint" style="margin-bottom:10px">
        Immer per UDP (unabhängig vom Protokoll oben - das betrifft nur die
        Status-Richtung). Ein einziger Virtueller Ausgang mit Adresse
        <span class="mono">/dev/udp/&lt;loxberry&gt;/&lt;Gateway-UDP-In-Port, Standard 11884&gt;</span>
        reicht, darunter je Zeile ein Virtueller Ausgang Befehl. "Befehl bei AUS"
        bleibt bei einmaligen Aktionen/Zahlenwerten leer. Diese Kommandos sind
        für JEDEN konfigurierten Charger IMMER aktiv (unabhängig von der
        Auswahl auf der "Daten"-Seite, die nur die Status-Topics betrifft).
      </div>
      <div style="overflow-x:auto; max-height:420px; overflow-y:auto">
        <table class="ga-table">
          <thead><tr><th>Bezeichnung (Mouseover: Payload/Wirkung)</th><th>Befehl bei EIN</th><th>Befehl bei AUS</th></tr></thead>
          <tbody id="loxone-cmd-tbody"></tbody>
        </table>
      </div>
    </div>
  </section>

  <section id="page-status" class="page">
    <h2>Status</h2>
    <div class="card">
      <div class="card-title-row">
        <div class="card-title">easeemqtt</div>
        <span class="card-toggle" id="toggle-svc" onclick="toggleLog('svc')">Log anzeigen &#9662;</span>
      </div>
      <div class="status-bar"><span class="status-dot" id="dot-svc2"></span><span id="txt-svc2">...</span></div>
      <pre class="log log-hidden" id="log-svc"></pre>
    </div>
    <div class="btn-group">
      <button class="btn btn-secondary" onclick="restartService()">Dienst neu starten</button>
    </div>
  </section>

  <section id="page-debug" class="page">
    <h2>Debug</h2>
    <div class="card">
      <div class="hint">UI-Version: <span id="ui-build"></span> - hilft zu prüfen, ob nach einer
        Neuinstallation tatsächlich der neue Stand geladen wurde (Browser-Cache!).</div>
    </div>
    <div class="card">
      <div class="card-title">Web-UI-Anfragen (Browser, dieser Tab)</div>
      <pre class="log" id="log-client"></pre>
    </div>
    <div class="card">
      <div class="card-title">api.cgi (Server)</div>
      <pre class="log" id="log-api"></pre>
    </div>
  </section>

</main>
</div>
<div class="toast" id="toast"></div>

<div class="modal-backdrop" id="charger-picker-backdrop" onclick="if(event.target===this) closeChargerPicker()">
  <div class="modal">
    <div class="card-title">Gefundene Ladegeräte</div>
    <div class="modal-list" id="charger-picker-list"></div>
    <div class="btn-group" style="margin-top:10px;margin-bottom:0">
      <button class="btn btn-secondary btn-sm" onclick="closeChargerPicker()">Schliessen</button>
    </div>
  </div>
</div>
</div>

<script>
const API = 'api.cgi';
let CHARGER_ROWS = [];
let FOUND_CHARGERS = [];

// Build-Marker: von Hand hochzählen bei relevanten Änderungen - gleiche
// Debug-Infrastruktur wie KNXtoLOX (dort entscheidend fürs schnelle
// Fehler-Isolieren auf echter Hardware).
const UI_BUILD = '2026-09-07-15';
console.log('[EaseeMQTT] index.cgi UI_BUILD=' + UI_BUILD);

const CLIENT_LOG = [];
function clientLog(line) {
  const ts = new Date().toLocaleTimeString('de-DE');
  CLIENT_LOG.push('[' + ts + '] ' + line);
  if (CLIENT_LOG.length > 200) CLIENT_LOG.shift();
  const el = document.getElementById('log-client');
  if (el) el.textContent = CLIENT_LOG.join('\n');
  console.log('[EaseeMQTT] ' + line);
}

// Rekursiv Passwörter/Tokens durch "***" ersetzen, BEVOR eine Antwort im
// (auf der Debug-Seite sichtbaren) Client-Log landet - action=config liefert
// easee.password/mqtt.password im Klartext zurück (das Formular braucht sie
// zum Vorausfüllen), die dürfen aber nicht zusätzlich im Log auftauchen.
// Server-seitiges Gegenstück: api.cgi's redact_for_log().
function redactForLog(data) {
  if (Array.isArray(data)) return data.map(redactForLog);
  if (data && typeof data === 'object') {
    const out = {};
    for (const k of Object.keys(data)) {
      out[k] = /pass|secret|token/i.test(k) ? '***' : redactForLog(data[k]);
    }
    return out;
  }
  return data;
}

// api() wirft nie - siehe KNXtoLOX für die Begründung (sonst stirbt bei
// Netzwerkfehler/nicht-JSON-Antwort die aufrufende async-Funktion lautlos).
async function api(action, opts) {
  opts = opts || {};
  const url = API + '?action=' + encodeURIComponent(action);
  const init = { method: opts.method || 'GET' };
  if (opts.body) { init.body = JSON.stringify(opts.body); init.headers = {'Content-Type':'application/json'}; }
  clientLog('-> ' + (init.method) + ' action=' + action);
  let res, text;
  try {
    res = await fetch(url, init);
    text = await res.text();
  } catch (e) {
    clientLog('<- ' + action + ': NETZWERKFEHLER ' + e.message);
    return { error: 'Netzwerkfehler: ' + e.message };
  }
  try {
    const data = JSON.parse(text);
    clientLog('<- ' + action + ': HTTP ' + res.status + ' ' + JSON.stringify(redactForLog(data)).slice(0, 200));
    return data;
  } catch (e) {
    clientLog('<- ' + action + ': HTTP ' + res.status + ', KEIN JSON: ' + text.slice(0, 150));
    return { error: 'Unerwartete Antwort vom Server (HTTP ' + res.status + ') - siehe Browser-Konsole (F12)' };
  }
}

function toast(msg, ok) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast show ' + (ok ? 'ok' : 'err');
  setTimeout(() => t.className = 'toast', 2500);
}

document.querySelectorAll('.nav-item').forEach(item => {
  item.addEventListener('click', () => {
    document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    item.classList.add('active');
    document.getElementById('page-' + item.dataset.page).classList.add('active');
    // Beim Wechsel auf "Loxone-Import" immer frisch aus dem aktuellen
    // Formularstand (Charger-Tabelle/Prefix/Daten-Auswahl) neu generieren -
    // die kann sich seit dem letzten Laden geändert haben, ohne dass
    // zwischendurch gespeichert wurde.
    if (item.dataset.page === 'loxone') renderLoxoneExport();
  });
});

function toggleSwitch(id) { setToggled(id, !isToggled(id)); applyLocalBrokerVisibility(); }
function isToggled(id) { const el = document.getElementById(id); return el && el.dataset.checked === 'true'; }
function setToggled(id, val) { const el = document.getElementById(id); if (el) el.dataset.checked = val ? 'true' : 'false'; }

function applyLocalBrokerVisibility() {
  const local = isToggled('use_local_broker');
  document.getElementById('local-broker-bar').style.display = local ? '' : 'none';
  document.getElementById('external-broker-fields').style.display = local ? 'none' : '';
}

// -- Wallboxen-Tabelle -----------------------------------------
function chargerRowHtml(row, idx) {
  row = row || {};
  const f = (k) => (row[k] || '').toString().replace(/"/g, '&quot;');
  return '<tr data-idx="' + idx + '">' +
    '<td><input type="text" data-role="none" data-f="id" value="' + f('id') + '" placeholder="z.B. EH1234AB"></td>' +
    '<td><input type="text" data-role="none" data-f="name" value="' + f('name') + '" placeholder="z.B. Carport links"></td>' +
    '<td><span class="ga-del" onclick="delChargerRow(' + idx + ')">&times;</span></td>' +
    '</tr>';
}

function renderChargerTable() {
  document.getElementById('charger-tbody').innerHTML = CHARGER_ROWS.map((r, i) => chargerRowHtml(r, i)).join('');
  document.getElementById('charger-summary').textContent = CHARGER_ROWS.length + ' Wallbox(en) konfiguriert';
}

function addChargerRow() { CHARGER_ROWS.push({}); renderChargerTable(); }
function delChargerRow(idx) { CHARGER_ROWS.splice(idx, 1); renderChargerTable(); }

function collectChargerRowsFromDom() {
  document.querySelectorAll('#charger-tbody tr').forEach(tr => {
    const idx = parseInt(tr.dataset.idx, 10);
    tr.querySelectorAll('[data-f]').forEach(el => { CHARGER_ROWS[idx][el.dataset.f] = el.value; });
  });
  return CHARGER_ROWS.filter(r => r.id);
}

// -- Observation-Katalog (Daten-Seite) --------------------------------
// Alle 170 bekannten Easee-Observation-IDs, generiert aus der echten
// ObservationID-Enum in evcc-io/evcc's Produktionscode (charger/easee/
// signalr.go) - siehe scratchpad/gen_observation_map.py (Generator, nicht
// von Hand abgetippt) und bridge.go's observationTopics für die
// server-seitige Entsprechung. "basic" markiert das vorausgewählte
// Basis-Setup (Ladezustand, Limits, Temperatur, Fehler, Kabel-Lock,
// Cloud-Status, alle Phasenströme/-spannungen).
const OBSERVATION_CATALOG = [
  {id:1,name:"self_test_result",group:"Basis / Selbsttest",basic:false},
  {id:2,name:"self_test_details",group:"Basis / Selbsttest",basic:false},
  {id:10,name:"wifi_event",group:"Basis / Selbsttest",basic:false},
  {id:11,name:"charger_offline_reason",group:"Basis / Selbsttest",basic:false},
  {id:13,name:"easee_link_command_response",group:"Basis / Selbsttest",basic:false},
  {id:14,name:"easee_link_data_received",group:"Basis / Selbsttest",basic:false},
  {id:15,name:"local_pre_authorize_enabled",group:"Basis / Selbsttest",basic:false},
  {id:16,name:"local_authorize_offline_enabled",group:"Basis / Selbsttest",basic:false},
  {id:17,name:"allow_offline_tx_for_unknown_id",group:"Basis / Selbsttest",basic:false},
  {id:18,name:"erratic_evmax_toggles",group:"Basis / Selbsttest",basic:false},
  {id:19,name:"backplate_type",group:"Basis / Selbsttest",basic:false},
  {id:20,name:"site_structure",group:"Standort & Circuit",basic:false},
  {id:21,name:"detected_power_grid_type",group:"Standort & Circuit",basic:false},
  {id:22,name:"circuit_max_current_p1",group:"Standort & Circuit",basic:false},
  {id:23,name:"circuit_max_current_p2",group:"Standort & Circuit",basic:false},
  {id:24,name:"circuit_max_current_p3",group:"Standort & Circuit",basic:false},
  {id:25,name:"location",group:"Standort & Circuit",basic:false},
  {id:26,name:"site_idstring",group:"Standort & Circuit",basic:false},
  {id:27,name:"site_idnumeric",group:"Standort & Circuit",basic:false},
  {id:28,name:"rfid_timeout_auth",group:"Standort & Circuit",basic:false},
  {id:30,name:"lock_cable_permanently",group:"Einstellungen & Verhalten",basic:false},
  {id:31,name:"is_enabled",group:"Einstellungen & Verhalten",basic:true},
  {id:32,name:"temperature_monitor_state",group:"Einstellungen & Verhalten",basic:false},
  {id:33,name:"circuit_sequence_number",group:"Einstellungen & Verhalten",basic:false},
  {id:34,name:"single_phase_number",group:"Einstellungen & Verhalten",basic:false},
  {id:35,name:"enable3_phases_deprecated",group:"Einstellungen & Verhalten",basic:false},
  {id:36,name:"wi_fi_ssid",group:"Einstellungen & Verhalten",basic:false},
  {id:37,name:"enable_idle_current",group:"Einstellungen & Verhalten",basic:false},
  {id:38,name:"phase_mode",group:"Einstellungen & Verhalten",basic:false},
  {id:39,name:"forced_three_phase_on_itwith_gnd_fault",group:"Einstellungen & Verhalten",basic:false},
  {id:40,name:"led_strip_brightness",group:"Einstellungen & Verhalten",basic:false},
  {id:41,name:"local_authorization_required",group:"Einstellungen & Verhalten",basic:false},
  {id:42,name:"authorization_required",group:"Einstellungen & Verhalten",basic:false},
  {id:43,name:"remote_start_required",group:"Einstellungen & Verhalten",basic:false},
  {id:44,name:"smart_button_enabled",group:"Einstellungen & Verhalten",basic:false},
  {id:45,name:"offline_charging_mode",group:"Einstellungen & Verhalten",basic:false},
  {id:46,name:"ledmode",group:"Einstellungen & Verhalten",basic:false},
  {id:47,name:"max_current_a",group:"Strom-Limits",basic:true},
  {id:48,name:"dynamic_current_a",group:"Strom-Limits",basic:true},
  {id:50,name:"max_current_offline_fallback_p1",group:"Strom-Limits",basic:false},
  {id:51,name:"max_current_offline_fallback_p2",group:"Strom-Limits",basic:false},
  {id:52,name:"max_current_offline_fallback_p3",group:"Strom-Limits",basic:false},
  {id:54,name:"release_cable_at_power_off",group:"Strom-Limits",basic:false},
  {id:56,name:"listen_to_control_pulse",group:"Strom-Limits",basic:false},
  {id:57,name:"control_pulse_rtt",group:"Strom-Limits",basic:false},
  {id:60,name:"charging_session_signed",group:"Ladeplan & Sessions",basic:false},
  {id:62,name:"charging_schedule",group:"Ladeplan & Sessions",basic:false},
  {id:65,name:"paired_equalizer",group:"Ladeplan & Sessions",basic:false},
  {id:68,name:"wi_fi_apenabled",group:"Ladeplan & Sessions",basic:false},
  {id:69,name:"paired_user_idtoken",group:"Ladeplan & Sessions",basic:false},
  {id:70,name:"circuit_total_allocated_phase_conductor_current_l1",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:71,name:"circuit_total_allocated_phase_conductor_current_l2",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:72,name:"circuit_total_allocated_phase_conductor_current_l3",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:73,name:"circuit_total_phase_conductor_current_l1",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:74,name:"circuit_total_phase_conductor_current_l2",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:75,name:"circuit_total_phase_conductor_current_l3",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:76,name:"number_of_cars_connected",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:77,name:"number_of_cars_charging",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:78,name:"number_of_cars_in_queue",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:79,name:"number_of_cars_fully_charged",group:"Circuit-Summen & Fahrzeuge",basic:false},
  {id:80,name:"software_release",group:"Hardware & Firmware",basic:false},
  {id:81,name:"iccid",group:"Hardware & Firmware",basic:false},
  {id:82,name:"modem_fw_id",group:"Hardware & Firmware",basic:false},
  {id:83,name:"otaerror_code",group:"Hardware & Firmware",basic:false},
  {id:84,name:"mobile_network_operator",group:"Hardware & Firmware",basic:false},
  {id:89,name:"reboot_reason",group:"Hardware & Firmware",basic:false},
  {id:90,name:"power_pcbversion",group:"Hardware & Firmware",basic:false},
  {id:91,name:"com_pcbversion",group:"Hardware & Firmware",basic:false},
  {id:96,name:"reason_for_no_current",group:"Diagnose",basic:false},
  {id:97,name:"load_balancing_number_of_connected_chargers",group:"Diagnose",basic:false},
  {id:98,name:"udpnum_of_connected_nodes",group:"Diagnose",basic:false},
  {id:99,name:"local_connection",group:"Diagnose",basic:false},
  {id:100,name:"pilot_mode",group:"Diagnose",basic:true},
  {id:101,name:"car_connected_deprecated",group:"Diagnose",basic:false},
  {id:102,name:"smart_charging",group:"Diagnose",basic:false},
  {id:103,name:"cable_locked",group:"Diagnose",basic:true},
  {id:104,name:"cable_rating",group:"Diagnose",basic:false},
  {id:105,name:"pilot_high",group:"Diagnose",basic:false},
  {id:106,name:"pilot_low",group:"Diagnose",basic:false},
  {id:107,name:"back_plate_id",group:"Diagnose",basic:false},
  {id:108,name:"user_idtoken_reversed",group:"Diagnose",basic:false},
  {id:109,name:"opmode",group:"Ladevorgang",basic:true},
  {id:110,name:"output_phase",group:"Ladevorgang",basic:true},
  {id:111,name:"dynamic_circuit_current_p1",group:"Ladevorgang",basic:false},
  {id:112,name:"dynamic_circuit_current_p2",group:"Ladevorgang",basic:false},
  {id:113,name:"dynamic_circuit_current_p3",group:"Ladevorgang",basic:false},
  {id:114,name:"output_current",group:"Ladevorgang",basic:false},
  {id:115,name:"derated_current",group:"Ladevorgang",basic:true},
  {id:116,name:"derating_active",group:"Ladevorgang",basic:true},
  {id:117,name:"debug_string",group:"Ladevorgang",basic:false},
  {id:118,name:"error_string",group:"Ladevorgang",basic:false},
  {id:119,name:"error_code",group:"Ladevorgang",basic:true},
  {id:120,name:"power_kw",group:"Ladevorgang",basic:true},
  {id:121,name:"session_energy_kwh",group:"Ladevorgang",basic:true},
  {id:122,name:"energy_per_hour",group:"Ladevorgang",basic:false},
  {id:123,name:"legacy_ev_status",group:"Ladevorgang",basic:false},
  {id:124,name:"lifetime_energy_kwh",group:"Ladevorgang",basic:true},
  {id:125,name:"lifetime_relay_switches",group:"Ladevorgang",basic:false},
  {id:126,name:"lifetime_hours",group:"Ladevorgang",basic:false},
  {id:127,name:"dynamic_current_offline_fallback_depricated",group:"Ladevorgang",basic:false},
  {id:128,name:"user_idtoken",group:"Ladevorgang",basic:false},
  {id:129,name:"charging_session",group:"Ladevorgang",basic:false},
  {id:130,name:"cell_rssi",group:"Mobilfunk & WLAN",basic:false},
  {id:131,name:"cell_rat",group:"Mobilfunk & WLAN",basic:false},
  {id:132,name:"wi_fi_rssi",group:"Mobilfunk & WLAN",basic:false},
  {id:133,name:"cell_address",group:"Mobilfunk & WLAN",basic:false},
  {id:134,name:"wi_fi_address",group:"Mobilfunk & WLAN",basic:false},
  {id:135,name:"wi_fi_type",group:"Mobilfunk & WLAN",basic:false},
  {id:136,name:"local_rssi",group:"Mobilfunk & WLAN",basic:false},
  {id:137,name:"master_back_plate_id",group:"Mobilfunk & WLAN",basic:false},
  {id:138,name:"local_tx_power",group:"Mobilfunk & WLAN",basic:false},
  {id:139,name:"local_state",group:"Mobilfunk & WLAN",basic:false},
  {id:140,name:"found_wi_fi",group:"Mobilfunk & WLAN",basic:false},
  {id:141,name:"current_connection",group:"Mobilfunk & WLAN",basic:false},
  {id:142,name:"cellular_interface_error_count",group:"Mobilfunk & WLAN",basic:false},
  {id:143,name:"cellular_interface_reset_count",group:"Mobilfunk & WLAN",basic:false},
  {id:144,name:"wifi_interface_error_count",group:"Mobilfunk & WLAN",basic:false},
  {id:145,name:"wifi_interface_reset_count",group:"Mobilfunk & WLAN",basic:false},
  {id:146,name:"local_node_type",group:"Mobilfunk & WLAN",basic:false},
  {id:147,name:"local_radio_channel",group:"Mobilfunk & WLAN",basic:false},
  {id:148,name:"local_short_address",group:"Mobilfunk & WLAN",basic:false},
  {id:149,name:"local_parent_addr_or_num_of_nodes",group:"Mobilfunk & WLAN",basic:false},
  {id:150,name:"temp_max",group:"Temperatur & Umgebung",basic:true},
  {id:151,name:"temp_ambient_power_board",group:"Temperatur & Umgebung",basic:false},
  {id:152,name:"temp_input_t2",group:"Temperatur & Umgebung",basic:false},
  {id:153,name:"temp_input_t3",group:"Temperatur & Umgebung",basic:false},
  {id:154,name:"temp_input_t4",group:"Temperatur & Umgebung",basic:false},
  {id:155,name:"temp_input_t5",group:"Temperatur & Umgebung",basic:false},
  {id:160,name:"temp_output_n",group:"Temperatur & Umgebung",basic:false},
  {id:161,name:"temp_output_l1",group:"Temperatur & Umgebung",basic:false},
  {id:162,name:"temp_output_l2",group:"Temperatur & Umgebung",basic:false},
  {id:163,name:"temp_output_l3",group:"Temperatur & Umgebung",basic:false},
  {id:170,name:"temp_ambient",group:"Temperatur & Umgebung",basic:false},
  {id:171,name:"light_ambient",group:"Temperatur & Umgebung",basic:false},
  {id:172,name:"int_rel_humidity",group:"Temperatur & Umgebung",basic:false},
  {id:173,name:"back_plate_locked",group:"Temperatur & Umgebung",basic:false},
  {id:174,name:"current_motor",group:"Temperatur & Umgebung",basic:false},
  {id:175,name:"back_plate_hall_sensor",group:"Temperatur & Umgebung",basic:false},
  {id:182,name:"current_t2",group:"Ströme & Spannungen",basic:true},
  {id:183,name:"current_t3",group:"Ströme & Spannungen",basic:true},
  {id:184,name:"current_t4",group:"Ströme & Spannungen",basic:true},
  {id:185,name:"current_t5",group:"Ströme & Spannungen",basic:true},
  {id:190,name:"in_volt_t1_t2",group:"Ströme & Spannungen",basic:true},
  {id:191,name:"in_volt_t1_t3",group:"Ströme & Spannungen",basic:true},
  {id:192,name:"in_volt_t1_t4",group:"Ströme & Spannungen",basic:true},
  {id:193,name:"in_volt_t1_t5",group:"Ströme & Spannungen",basic:true},
  {id:194,name:"in_volt_t2_t3",group:"Ströme & Spannungen",basic:true},
  {id:195,name:"in_volt_t2_t4",group:"Ströme & Spannungen",basic:true},
  {id:196,name:"in_volt_t2_t5",group:"Ströme & Spannungen",basic:true},
  {id:197,name:"in_volt_t3_t4",group:"Ströme & Spannungen",basic:true},
  {id:198,name:"in_volt_t3_t5",group:"Ströme & Spannungen",basic:true},
  {id:199,name:"in_volt_t4_t5",group:"Ströme & Spannungen",basic:true},
  {id:202,name:"out_volt_pin1_2",group:"Ströme & Spannungen",basic:false},
  {id:203,name:"out_volt_pin1_3",group:"Ströme & Spannungen",basic:false},
  {id:204,name:"out_volt_pin1_4",group:"Ströme & Spannungen",basic:false},
  {id:205,name:"out_volt_pin1_5",group:"Ströme & Spannungen",basic:false},
  {id:206,name:"out_volt_pin2_3",group:"Ströme & Spannungen",basic:false},
  {id:210,name:"volt_level33",group:"Ströme & Spannungen",basic:false},
  {id:211,name:"volt_level5",group:"Ströme & Spannungen",basic:false},
  {id:212,name:"volt_level12",group:"Ströme & Spannungen",basic:false},
  {id:219,name:"fatal_error_code",group:"LTE & Sonstiges",basic:true},
  {id:220,name:"lte_rsrp",group:"LTE & Sonstiges",basic:false},
  {id:221,name:"lte_sinr",group:"LTE & Sonstiges",basic:false},
  {id:222,name:"lte_rsrq",group:"LTE & Sonstiges",basic:false},
  {id:223,name:"charge_session_start",group:"LTE & Sonstiges",basic:false},
  {id:230,name:"eq_available_current_p1",group:"LTE & Sonstiges",basic:false},
  {id:231,name:"eq_available_current_p2",group:"LTE & Sonstiges",basic:false},
  {id:232,name:"eq_available_current_p3",group:"LTE & Sonstiges",basic:false},
  {id:250,name:"connected_to_cloud",group:"LTE & Sonstiges",basic:true},
  {id:251,name:"cloud_disconnect_reason",group:"LTE & Sonstiges",basic:true}
];

let SELECTED_OBS = new Set();

function obsBasicIds() {
  return OBSERVATION_CATALOG.filter(o => o.basic).map(o => o.id);
}

function setObsSelection(mode) {
  if (mode === 'basic') SELECTED_OBS = new Set(obsBasicIds());
  else if (mode === 'all') SELECTED_OBS = new Set(OBSERVATION_CATALOG.map(o => o.id));
  else SELECTED_OBS = new Set();
  renderObsList();
}

function toggleObs(id) {
  if (SELECTED_OBS.has(id)) SELECTED_OBS.delete(id); else SELECTED_OBS.add(id);
  renderObsList();
}

function renderObsList() {
  const q = (document.getElementById('obs-search').value || '').toLowerCase().trim();
  const filtered = OBSERVATION_CATALOG.filter(o =>
    !q || o.name.includes(q) || String(o.id) === q || o.group.toLowerCase().includes(q)
  );
  const groups = {};
  filtered.forEach(o => { (groups[o.group] = groups[o.group] || []).push(o); });

  const html = Object.keys(groups).map(g => {
    const items = groups[g];
    const selCount = items.filter(o => SELECTED_OBS.has(o.id)).length;
    const rows = items.map(o =>
      '<div class="obs-row" onclick="toggleObs(' + o.id + ')">' +
        '<div class="obs-check" data-checked="' + SELECTED_OBS.has(o.id) + '"></div>' +
        '<span class="obs-id">' + o.id + '</span>' +
        '<span class="obs-name">' + o.name + '</span>' +
      '</div>'
    ).join('');
    return '<div class="obs-group">' +
      '<div class="obs-group-title"><span>' + g + '</span><span class="obs-group-count">' + selCount + '/' + items.length + '</span></div>' +
      '<div class="obs-grid">' + rows + '</div></div>';
  }).join('');
  document.getElementById('obs-groups').innerHTML = html || '<div class="hint">keine Treffer</div>';
  document.getElementById('obs-count').textContent = SELECTED_OBS.size + ' von ' + OBSERVATION_CATALOG.length + ' ausgewählt';
}

// -- Konfiguration laden/speichern -------------------------------
function currentConfigForm() {
  return {
    easee: {
      username: document.getElementById('easee_username').value,
      password: document.getElementById('easee_password').value,
      chargers: collectChargerRowsFromDom(),
    },
    mqtt: {
      use_local_broker: isToggled('use_local_broker'),
      host: document.getElementById('mqtt_host').value,
      port: parseInt(document.getElementById('mqtt_port').value || '1883', 10),
      username: document.getElementById('mqtt_username').value,
      password: document.getElementById('mqtt_password').value,
      topic_prefix: document.getElementById('topic_prefix').value || 'easee/',
      client_id: document.getElementById('client_id').value || 'easeemqtt',
      enabled_observations: Array.from(SELECTED_OBS),
    },
  };
}

function fillConfigForm(c) {
  const e = c.easee || {};
  document.getElementById('easee_username').value = e.username || '';
  document.getElementById('easee_password').value = e.password || '';
  CHARGER_ROWS = e.chargers || [];
  renderChargerTable();

  const m = c.mqtt || {};
  setToggled('use_local_broker', m.use_local_broker !== false);
  document.getElementById('mqtt_host').value = m.host || '';
  document.getElementById('mqtt_port').value = m.port || 1883;
  document.getElementById('mqtt_username').value = m.username || '';
  document.getElementById('mqtt_password').value = m.password || '';
  document.getElementById('topic_prefix').value = m.topic_prefix || 'easee/';
  document.getElementById('client_id').value = m.client_id || 'easeemqtt';
  document.getElementById('local-broker-info').textContent = 'MQTT-Broker (aus LoxBerry): ' + (c.local_broker_info || 'nicht konfiguriert');
  applyLocalBrokerVisibility();

  // enabled_observations fehlt nur bei einer ganz frischen, noch nie
  // gespeicherten Konfiguration (config.json.default liefert es bereits mit)
  // - dann auf das Basis-Setup zurückfallen statt auf eine leere Auswahl.
  SELECTED_OBS = new Set(Array.isArray(m.enabled_observations) ? m.enabled_observations : obsBasicIds());
  renderObsList();

  renderTopicExamples(m.topic_prefix || 'easee/', CHARGER_ROWS);
  renderLoxoneExport();
}

// Zeigt die tatsächlich konfigurierten Prefix+Charger-IDs auf der
// Übersicht-Seite, damit man die vollständigen Topic-Pfade nicht erst aus
// Platzhaltern ("<prefix><charger_id>/...") zusammenbauen muss - direkte
// Antwort auf "wo sehe ich die Befehle, die ich senden kann".
function renderTopicExamples(prefix, chargers) {
  const el = document.getElementById('topic-examples');
  if (!el) return;
  if (!chargers || chargers.length === 0) {
    el.textContent = 'Noch keine Wallbox konfiguriert - siehe "Konto & Wallboxen".';
    return;
  }
  el.innerHTML = chargers.map(c => {
    const id = (c.id || '').replace(/</g, '&lt;');
    const name = c.name ? ' (' + c.name.replace(/</g, '&lt;') + ')' : '';
    return '<div style="margin-bottom:6px">' +
      '<strong>' + id + '</strong>' + name + '<br>' +
      '<span class="mono" style="font-size:12px">' + prefix + id + '/state/opmode</span> ... , ' +
      '<span class="mono" style="font-size:12px">' + prefix + id + '/cmd/start</span>, ' +
      '<span class="mono" style="font-size:12px">' + prefix + id + '/set/dynamic_current</span> ...' +
    '</div>';
  }).join('');
}

// -- Loxone-Import: fertige Copy-Paste-Strings für LoxBerrys MQTT-Gateway --
// Format/Syntax verifiziert gegen die echten LoxBerry-Wiki-Anleitungen
// (wiki.loxberry.de, .../mqtt_schritt_fur_schritt_mqtt_loxone für die
// Status-Richtung und .../mqtt_schritt_fur_schritt_loxone_mqtt für die
// Kommando-Richtung, beide 2026-09-07 gegengeprüft) - nicht geraten:
// - MQTT -> Loxone (unsere State-Topics) läuft über das Gateway-Plugin,
//   das Topic muss dort im Tab "Abonnements" erst abonniert werden (das
//   kann diese Seite nicht automatisieren). Bei UDP-Protokoll erkennt ein
//   "Virtueller UDP Eingang Befehl" das Muster "MQTT:\i<topic>=\i\v"
//   wörtlich (\i = Trennzeichen, \v = Wert) - Topic bleibt mit Schrägstrichen
//   unverändert. Bei HTTP-Protokoll muss die Bezeichnung des Virtuellen
//   Eingangs EXAKT dem Topic mit Schrägstrichen->Unterstrichen entsprechen.
// - Loxone -> MQTT (unsere Kommando-Topics) läuft immer per UDP an einen
//   Virtuellen Ausgang (Adresse /dev/udp/<loxberry>/<Gateway-UDP-In-Port>),
//   Befehl bei EIN/AUS lautet "publish <topic> <wert>" bzw. mit \v für
//   Analogwerte.
// payload/desc = dieselben Erklärungen, die früher in der (jetzt entfernten,
// redundanten) Kommando-Tabelle auf der Übersicht-Seite standen - hierher
// verschoben, damit sie nicht verloren gehen.
const COMMAND_CATALOG = [
  {topic:'cmd/start', label:'start', kind:'trigger', payload:'beliebig', desc:'Laden starten'},
  {topic:'cmd/stop', label:'stop', kind:'trigger', payload:'beliebig', desc:'Laden stoppen'},
  {topic:'cmd/pause', label:'pause', kind:'trigger', payload:'beliebig', desc:'Laden pausieren'},
  {topic:'cmd/resume', label:'resume', kind:'trigger', payload:'beliebig', desc:'Pausiertes Laden fortsetzen'},
  {topic:'cmd/toggle', label:'toggle', kind:'trigger', payload:'beliebig', desc:'Start/Stopp umschalten'},
  {topic:'cmd/reboot', label:'reboot', kind:'trigger', payload:'beliebig', desc:'Wallbox neu starten'},
  {topic:'cmd/override_schedule', label:'override_schedule', kind:'trigger', payload:'beliebig', desc:'Easee-eigenen Zeitplan ignorieren, sofort laden'},
  {topic:'set/dynamic_current', label:'set_dynamic_current', kind:'analog', payload:'Zahl (A)', desc:'Flüchtiges Strom-Limit (P1/P2/P3 gleich), Hebel für externes Lastmanagement'},
  {topic:'set/max_current', label:'set_max_current', kind:'analog', payload:'Ganzzahl (A)', desc:'Dauerhaftes Strom-Limit'},
  {topic:'set/enabled', label:'set_enabled', kind:'bool', payload:'0/1', desc:'Charger komplett aktivieren/deaktivieren'},
  {topic:'set/cable_lock', label:'set_cable_lock', kind:'bool', payload:'0/1', desc:'Kabel dauerhaft verriegeln'},
  {topic:'set/single_phase_limit', label:'set_single_phase_limit', kind:'bool', payload:'0/1', desc:'Auf 1-phasiges Laden begrenzen'},
  {topic:'set/phase_mode', label:'set_phase_mode', kind:'analog', payload:'1/2/3', desc:'1=1-phasig fest, 2=Auto, 3=3-phasig fest'},
  {topic:'set/smart_charging', label:'set_smart_charging', kind:'bool', payload:'0/1', desc:'Smart-Charging-Flag'},
  {topic:'set/led_brightness', label:'set_led_brightness', kind:'analog', payload:'0-100', desc:'LED-Streifen-Helligkeit'},
  {topic:'set/idle_current', label:'set_idle_current', kind:'bool', payload:'0/1', desc:'Reststrom nach Ladeende signalisieren'},
];

// "Topic: Schrägstriche, Leerzeichen und % werden zu Unterstrichen" (HTTP-
// Namensregel laut LoxBerry-Wiki) - unsere Topics enthalten nie Leerzeichen/%,
// nur Schrägstriche.
function httpName(topic) {
  return topic.replace(/[\/ %]/g, '_');
}

// Schreibgeschütztes Eingabefeld für eine Tabellenzelle - onclick statt
// nur readonly-Text, damit ein einziger Klick GENAU diesen einen Wert
// auswählt (Strg+C kopiert dann nur das, nicht die ganze Zeile/Tabelle).
// Getrennt von Bezeichnung, weil in Loxone Config Bezeichnung und Befehl(-
// serkennung) grundsätzlich zwei verschiedene Formularfelder sind und die
// Objekte ohnehin einzeln nacheinander angelegt werden müssen - ein grosser
// Textblock zum "alles auf einmal kopieren" half hier also nicht.
function roCell(value, title) {
  const v = (value || '').replace(/"/g, '&quot;');
  const t = title ? ' title="' + title.replace(/"/g, '&quot;') + '"' : '';
  return '<td><input type="text" class="mono" readonly data-role="none" value="' + v + '"' + t + ' onclick="this.select()"></td>';
}

// Baut die Wallbox-Auswahl auf der Loxone-Import-Seite neu auf - eigene
// Funktion statt inline, damit die aktuelle Auswahl beim Neu-Rendern
// erhalten bleibt (z.B. wenn nur der Protokoll-Schalter umgestellt wird),
// solange die zuvor gewaehlte Charger-ID noch existiert.
function refreshLoxoneChargerOptions(sel, chargers) {
  const current = sel.value;
  sel.innerHTML = '<option value="">Alle Wallboxen</option>' + chargers.map(c =>
    '<option value="' + c.id.replace(/"/g, '&quot;') + '">' + (c.name || c.id).replace(/</g, '&lt;') + '</option>'
  ).join('');
  if (chargers.some(c => c.id === current)) sel.value = current;
}

function renderLoxoneExport() {
  const protocol = document.getElementById('loxone-protocol').value;
  const prefix = document.getElementById('topic_prefix').value || 'easee/';
  const allChargers = collectChargerRowsFromDom();
  const chargerSelect = document.getElementById('loxone-charger');
  refreshLoxoneChargerOptions(chargerSelect, allChargers);
  // Leere Auswahl ("") = alle Wallboxen (bisheriges Verhalten), sonst nur
  // die eine gewaehlte - haelt die Tabellen bei mehreren Wallboxen uebersichtlich.
  const chargers = chargerSelect.value ? allChargers.filter(c => c.id === chargerSelect.value) : allChargers;
  const selected = OBSERVATION_CATALOG.filter(o => SELECTED_OBS.has(o.id));

  const hintEl = document.getElementById('loxone-state-hint');
  const theadEl = document.getElementById('loxone-state-thead');
  const stateBodyEl = document.getElementById('loxone-state-tbody');
  const cmdBodyEl = document.getElementById('loxone-cmd-tbody');

  if (chargers.length === 0) {
    theadEl.innerHTML = '';
    stateBodyEl.innerHTML = cmdBodyEl.innerHTML = '';
    hintEl.textContent = 'Noch keine Wallbox konfiguriert - siehe "Konto & Wallboxen".';
    return;
  }

  if (protocol === 'udp') {
    hintEl.textContent = 'Für je eine Zeile: Virtueller UDP Eingang Befehl anlegen - "Bezeichnung" frei wählbar (Vorschlag in Spalte 1), "Befehlserkennung" = Spalte 2.';
    theadEl.innerHTML = '<th>Bezeichnung</th><th>Befehlserkennung</th>';
    const rows = [];
    chargers.forEach(c => {
      selected.forEach(o => {
        const topic = prefix + c.id + '/state/' + o.name;
        rows.push('<tr>' + roCell((c.name || c.id) + ' ' + o.name) + roCell('MQTT:\\i' + topic + '=\\i\\v') + '</tr>');
      });
    });
    stateBodyEl.innerHTML = rows.join('');
  } else {
    hintEl.textContent = 'Für je eine Zeile: Virtueller Eingang anlegen - "Bezeichnung" muss EXAKT diesem Wert entsprechen (kopieren, nicht abtippen). "Als Digitaleingang verwenden" NICHT aktivieren.';
    theadEl.innerHTML = '<th>Bezeichnung (= Virtueller Eingang Name)</th>';
    const rows = [];
    chargers.forEach(c => {
      selected.forEach(o => {
        const topic = prefix + c.id + '/state/' + o.name;
        rows.push('<tr>' + roCell(httpName(topic)) + '</tr>');
      });
    });
    stateBodyEl.innerHTML = rows.join('');
  }

  const cmdRows = [];
  chargers.forEach(c => {
    COMMAND_CATALOG.forEach(cmd => {
      const topic = prefix + c.id + '/' + cmd.topic;
      const label = (c.name || c.id) + ' ' + cmd.label;
      const title = 'Payload: ' + cmd.payload + ' - Wirkung: ' + cmd.desc;
      if (cmd.kind === 'trigger') {
        cmdRows.push('<tr>' + roCell(label, title) + roCell('publish ' + topic + ' 1') + roCell('') + '</tr>');
      } else if (cmd.kind === 'analog') {
        cmdRows.push('<tr>' + roCell(label, title) + roCell('publish ' + topic + ' \\v') + roCell('') + '</tr>');
      } else {
        cmdRows.push('<tr>' + roCell(label, title) + roCell('publish ' + topic + ' 1') + roCell('publish ' + topic + ' 0') + '</tr>');
      }
    });
  });
  cmdBodyEl.innerHTML = cmdRows.join('');
}

async function loadConfig() {
  const c = await api('config');
  fillConfigForm(c);
}

async function saveConfig() {
  const body = currentConfigForm();
  const r = await api('config', { method: 'POST', body });
  toast(r.ok ? 'Gespeichert - Dienst wird neu gestartet ...' : (r.error || 'Fehler'), !!r.ok);
}

// Ein erfolgreicher Login verrät Easee bereits die komplette Geräteliste
// des Kontos - es braucht dafür keinen separaten zweiten Klick. testEasee()
// zeigt den gefundenen Bestand deshalb direkt im Picker an, sobald der Login
// klappt (searchChargers() bleibt als manuelles "neu laden" verfügbar, z.B.
// wenn seither ein neuer Charger im Easee-Konto dazukam).
async function testEasee() {
  const body = { username: document.getElementById('easee_username').value, password: document.getElementById('easee_password').value };
  const r = await api('easee_test', { method: 'POST', body });
  const el = document.getElementById('easee-test-status');
  el.textContent = r.ok ? 'Login OK' : (r.error || 'Login fehlgeschlagen');
  toast(r.ok ? 'Easee-Login OK' : (r.error || 'Login fehlgeschlagen'), !!r.ok);
  if (!r.ok) return;
  FOUND_CHARGERS = r.chargers || [];
  if (r.chargers_error) {
    toast(r.chargers_error, false);
  } else if (FOUND_CHARGERS.length === 0) {
    toast('Login OK, aber keine Ladegeräte auf diesem Konto gefunden', false);
  } else {
    renderChargerPicker();
    document.getElementById('charger-picker-backdrop').classList.add('show');
  }
}

// -- Ladegeräte-Picker ----------------------------------------
async function searchChargers() {
  const body = { username: document.getElementById('easee_username').value, password: document.getElementById('easee_password').value };
  const r = await api('easee_chargers', { method: 'POST', body });
  if (r.error) { toast(r.error, false); return; }
  FOUND_CHARGERS = r.chargers || [];
  if (FOUND_CHARGERS.length === 0) { toast('Keine Ladegeräte auf diesem Konto gefunden', false); return; }
  renderChargerPicker();
  document.getElementById('charger-picker-backdrop').classList.add('show');
}

function closeChargerPicker() {
  document.getElementById('charger-picker-backdrop').classList.remove('show');
}

function renderChargerPicker() {
  collectChargerRowsFromDom();
  const already = new Set(CHARGER_ROWS.map(r => r.id));
  const list = document.getElementById('charger-picker-list');
  list.innerHTML = FOUND_CHARGERS.map((c, i) => {
    const added = already.has(c.id);
    return '<div class="modal-item">' +
      '<div><div class="name">' + c.name.replace(/</g, '&lt;') + '</div><div class="meta">' + c.id + '</div></div>' +
      '<button type="button" class="btn btn-secondary btn-sm" ' + (added ? 'disabled' : 'onclick="pickCharger(' + i + ')"') + '>' + (added ? 'bereits hinzugefügt' : '+ hinzufügen') + '</button>' +
    '</div>';
  }).join('');
}

function pickCharger(i) {
  const c = FOUND_CHARGERS[i];
  if (!c) return;
  collectChargerRowsFromDom();
  if (!CHARGER_ROWS.some(r => r.id === c.id)) {
    CHARGER_ROWS.push({ id: c.id, name: c.name });
    renderChargerTable();
  }
  renderChargerPicker();
}

// -- Status ----------------------------------------------------
function toggleLog(name) {
  const pre = document.getElementById('log-' + name);
  const btn = document.getElementById('toggle-' + name);
  if (!pre || !btn) return;
  const show = pre.classList.toggle('log-hidden') === false;
  btn.textContent = show ? 'Log verbergen ^' : 'Log anzeigen v';
}

function applyServiceStatus(s) {
  const ok = !!s.running;
  document.querySelectorAll('#dot-svc, #dot-svc2').forEach(el => el.className = 'status-dot ' + (ok ? 'ok' : 'err'));
  const txt = (ok ? 'aktiv' : 'inaktiv') + (s.pid ? ' (PID ' + s.pid + ')' : '');
  document.getElementById('txt-svc').textContent = 'easeemqtt: ' + txt;
  document.getElementById('txt-svc2').textContent = txt;
  const log = document.getElementById('log-svc');
  if (log && s.log !== undefined) log.textContent = s.log;
}

async function restartService() {
  const r = await api('service_restart', { method: 'POST' });
  toast(r.ok ? 'Dienst neu gestartet' : 'Fehler beim Neustart', !!r.ok);
}

async function pollStatus() {
  const s = await api('service_status');
  if (!s.error) applyServiceStatus(s);
  const l = await api('api_log');
  const el = document.getElementById('log-api');
  if (el && l.log !== undefined) el.textContent = l.log;
}

async function step(name, fn) {
  try { await fn(); }
  catch (e) {
    console.error('init-Schritt "' + name + '" fehlgeschlagen:', e);
    toast('Fehler beim Laden (' + name + '): ' + e.message, false);
  }
}

(async function init() {
  document.getElementById('ui-build').textContent = UI_BUILD;
  await step('config', loadConfig);
  await step('status', pollStatus);
  setInterval(pollStatus, 5000);
})();
</script>
HTML

LoxBerry::Web::lbfooter();
