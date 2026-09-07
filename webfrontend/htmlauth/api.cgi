#!/usr/bin/perl
# EaseeMQTT API - JSON-Action-Dispatch für die Single-Page-App (index.cgi).
# Struktur/Konventionen 1:1 vom Schwester-Plugin KNXtoLOX übernommen (dort
# real-hardware-verifiziert, siehe dessen api.cgi/CHANGELOG): eigenes
# applog() statt LoxBerry::Log (dessen Methodennamen dort nachweislich falsch
# geraten wurden), url_param()-Routing (param() mischt Query/Body bei
# JSON-POSTs nicht zuverlässig), JSON::PP->utf8 durchgehend, run_capture()
# für jeden system()-Aufruf (STDOUT/STDERR eines Kindprozesses landet sonst
# im HTTP-Response-Body).
use LoxBerry::System;
use LoxBerry::Web;
use CGI;
use JSON::PP;
use strict;
use warnings;

my $cgi = CGI->new;

sub run_capture {
    my (@cmd) = @_;
    my $capfile = "/tmp/easeemqtt_run_$$.out";
    open(my $oldout, '>&', \*STDOUT) or return (system(@cmd), '');
    open(my $olderr, '>&', \*STDERR) or return (system(@cmd), '');
    open(STDOUT, '>', $capfile) or return (system(@cmd), '');
    open(STDERR, '>&', \*STDOUT);
    my $rc = system(@cmd);
    open(STDOUT, '>&', $oldout);
    open(STDERR, '>&', $olderr);
    close $oldout; close $olderr;
    my $output = '';
    if (open(my $cf, '<', $capfile)) { local $/; $output = <$cf>; close $cf; }
    unlink $capfile;
    return ($rc, $output);
}

# Wie run_capture(), sendet aber zusaetzlich $stdin_text auf STDIN des
# Kindprozesses - fuer encrypt_secret()/decrypt_secret() (siehe unten), damit
# ein Passwort NICHT als Kommandozeilenargument uebergeben werden muss (sonst
# fuer die Dauer des Aufrufs in der Prozessliste/ps aux sichtbar). Gleiches
# FD-Umbiege-Muster wie run_capture(), nur um STDIN erweitert - bewusst kein
# IPC::Open2 o.ae., um keine zusaetzliche Modul-Abhaengigkeit einzufuehren.
sub run_capture_stdin {
    my ($cmd_ref, $stdin_text) = @_;
    my @cmd = @$cmd_ref;
    my $infile = "/tmp/easeemqtt_stdin_$$.tmp";
    open(my $ifh, '>', $infile) or return (1, '');
    chmod 0600, $infile;
    print $ifh $stdin_text;
    close $ifh;

    my $capfile = "/tmp/easeemqtt_run_$$.out";
    open(my $oldin, '<&', \*STDIN) or do { unlink $infile; return (system(@cmd), ''); };
    open(my $oldout, '>&', \*STDOUT) or do { unlink $infile; return (system(@cmd), ''); };
    open(my $olderr, '>&', \*STDERR) or do { unlink $infile; return (system(@cmd), ''); };
    open(STDIN, '<', $infile) or do { unlink $infile; return (system(@cmd), ''); };
    open(STDOUT, '>', $capfile) or do { unlink $infile; return (system(@cmd), ''); };
    open(STDERR, '>&', \*STDOUT);
    my $rc = system(@cmd);
    open(STDIN, '<&', $oldin);
    open(STDOUT, '>&', $oldout);
    open(STDERR, '>&', $olderr);
    close $oldin; close $oldout; close $olderr;
    my $output = '';
    if (open(my $cf, '<', $capfile)) { local $/; $output = <$cf>; close $cf; }
    unlink $capfile;
    unlink $infile;
    return ($rc, $output);
}

# Eigenes, simples Datei-Logging statt LoxBerry::Log's benannter Methoden -
# siehe KNXtoLOX/api.cgi für die Begründung (zwei geratene Methodennamen,
# beide nachweislich falsch). Reines Perl, garantiert lauffähig.
sub applog {
    my ($msg) = @_;
    my @t = localtime();
    my $ts = sprintf('%04d-%02d-%02d %02d:%02d:%02d', $t[5]+1900, $t[4]+1, $t[3], $t[2], $t[1], $t[0]);
    if (open(my $fh, '>>:encoding(UTF-8)', "$lbplogdir/api.log")) {
        print $fh "[$ts] $msg\n";
        close $fh;
    }
}

my $cfgfile     = "$lbpconfigdir/config.json"; # von easeemqtt (Go-Daemon) DIREKT gelesen - siehe README
my $enginelogfile = "$lbplogdir/easeemqtt.log";
my $SERVICE     = 'easeemqtt.service';
my $EASEEMQTT_BIN = '/opt/loxberry/bin/plugins/easeemqtt/easeemqtt';

print $cgi->header(-type => 'application/json', -charset => 'utf-8', 'Cache-Control' => 'no-store');

sub out { print JSON::PP->new->utf8->canonical->encode($_[0]); exit 0; }
sub err { my ($msg) = @_; applog($msg); out({ error => $msg }); }

# -- LoxBerry-Systemkonfiguration direkt lesen (general.json) ---------------
# Gleicher Pfad wie MiraiPanel/bin/bridge.js UND KNXtoLOX/api.cgi (dort beide
# nachweislich funktionierend): NICHT über ein LoxBerry::System-Perl-Global,
# sondern der hart hinterlegte Pfad ${LBHOMEDIR:-/opt/loxberry}/config/system/
# general.json.
sub read_general_json {
    my $home = $ENV{LBHOMEDIR} || '/opt/loxberry';
    my $file = "$home/config/system/general.json";
    return {} unless -f $file;
    local $/;
    open(my $fh, '<', $file) or return {};
    my $text = <$fh>;
    close $fh;
    my $data = eval { JSON::PP->new->utf8->decode($text) };
    return $data || {};
}

sub mqtt_broker_conn {
    my $general = read_general_json();
    my $m = $general->{Mqtt} || {};
    return undef unless $m->{Brokerhost};
    return {
        host => $m->{Brokerhost},
        port => $m->{Brokerport} || 1883,
        user => $m->{Brokeruser} || '',
        pass => $m->{Brokerpass} || '',
    };
}

# -- Passwort-Verschluesselung -----------------------------------------------
# Ruft den easeemqtt-Daemon selbst als Subprocess auf ("easeemqtt encrypt/
# decrypt <config-datei>") statt Krypto in Perl zu duplizieren - die
# eigentliche AES-256-GCM-Implementierung lebt einmalig in Gos
# Standardbibliothek (daemon/internal/config/crypto.go), Schluessel liegt
# lazy neben config.json (secret.key, Modus 0600). Wert wird ueber STDIN
# uebergeben (run_capture_stdin), nie als Kommandozeilenargument - sonst
# waere das Passwort kurzzeitig in der Prozessliste sichtbar.
sub crypto_call {
    my ($subcmd, $stdin_text) = @_;
    my ($rc, $out) = run_capture_stdin([$EASEEMQTT_BIN, $subcmd, $cfgfile], $stdin_text);
    return undef if $rc != 0;
    $out =~ s/\r?\n\z//;
    return $out;
}

# Verschluesselt ein neu eingegebenes Passwort vor dem Schreiben in
# config.json. Leerer Wert bleibt leer (kein Passwort gesetzt). Schlaegt der
# Aufruf fehl (z.B. Binary noch nicht gebaut), wird laut und sichtbar
# gefehlert statt still Klartext zu speichern - das waere schlimmer als ein
# fehlgeschlagener Speichervorgang.
sub encrypt_secret {
    my ($plain) = @_;
    return '' unless length($plain // '');
    my $enc = crypto_call('encrypt', $plain);
    err('Verschluesselung fehlgeschlagen - ist der Dienst korrekt installiert?') unless defined($enc);
    return $enc;
}

# Entschluesselt einen gespeicherten Wert NUR fuer den internen
# Login-Test-Fallback (siehe easee_test/easee_chargers) - wird niemals an den
# Browser zurueckgegeben. Schlaegt die Entschluesselung fehl (z.B. weil der
# Wert aus einer aelteren, noch unverschluesselten config.json stammt),
# unveraendert als moeglichen Klartext zurueckgeben statt hart zu fehlern -
# gleiche Uebergangs-Logik wie im Go-Daemon (siehe dessen config.go).
sub decrypt_secret {
    my ($stored) = @_;
    return '' unless length($stored // '');
    my $plain = crypto_call('decrypt', $stored);
    return defined($plain) ? $plain : $stored;
}

# -- config.json laden/speichern ---------------------------------------------
# enabled_observations-Default = dasselbe Basis-Set wie config/
# config.json.default (Nutzer-Vorgabe 2026-09-07: Checkbox-Liste, vorausgewählt
# mit sinnvollem Basisumfang statt "alles" oder "nichts") - Ladezustand,
# Strom-Limits, Temperatur, Fehler-Codes, Kabel-Lock, Cloud-Status, alle
# Phasenströme/-spannungen. Vollständige Liste/Kategorien siehe
# OBSERVATION_CATALOG in index.cgi.
my @DEFAULT_ENABLED_OBSERVATIONS = (31, 47, 48, 100, 103, 109, 110, 115, 116, 119, 120, 121, 124, 150,
    182, 183, 184, 185, 190, 191, 192, 193, 194, 195, 196, 197, 198, 199, 219, 250, 251);

my %CONFIG_DEFAULTS = (
    easee => { username => '', password => '', chargers => [] },
    mqtt  => {
        use_local_broker => JSON::PP::true,
        host => '', port => 1883, username => '', password => '',
        topic_prefix => 'easee/', client_id => 'easeemqtt',
        enabled_observations => [ @DEFAULT_ENABLED_OBSERVATIONS ],
    },
);

sub load_config {
    my %c = %CONFIG_DEFAULTS;
    if (-f $cfgfile) {
        local $/;
        open(my $fh, '<', $cfgfile) or return \%c;
        my $data = eval { JSON::PP->new->utf8->decode(<$fh>) };
        close $fh;
        if ($data) {
            for my $key (keys %$data) { $c{$key} = $data->{$key}; }
        }
    }
    return \%c;
}

sub save_config {
    my ($body) = @_;
    my %c = (%CONFIG_DEFAULTS, %$body);
    $c{easee} = { %{ $CONFIG_DEFAULTS{easee} }, %{ $body->{easee} || {} } };
    $c{mqtt}  = { %{ $CONFIG_DEFAULTS{mqtt} },  %{ $body->{mqtt}  || {} } };

    # Bereits gespeicherte (verschluesselte) Werte laden - fuer "Feld leer
    # gelassen bedeutet unveraendert" bei den Passwoertern unten.
    my $existing = load_config();

    # easee.password: leeres Feld = Frontend hat es absichtlich leer
    # gelassen (siehe index.cgi/fillConfigForm - Passwoerter werden nie an
    # den Browser zurueckgegeben), bestehenden verschluesselten Wert
    # behalten. Nicht-leerer Wert = neues Passwort, frisch verschluesseln.
    if (length($c{easee}{password} // '')) {
        $c{easee}{password} = encrypt_secret($c{easee}{password});
    } else {
        $c{easee}{password} = $existing->{easee}{password} // '';
    }

    # use_local_broker=true: host/port/user/passwort IMMER frisch aus
    # LoxBerrys general.json auflösen und in config.json (die einzige, vom
    # Go-Daemon direkt gelesene Datei) schreiben - der Daemon selbst kennt
    # general.json/LoxBerry-Spezifika nicht, siehe daemon/internal/config.
    if ($c{mqtt}{use_local_broker}) {
        my $broker = mqtt_broker_conn();
        if ($broker) {
            $c{mqtt}{host} = $broker->{host};
            $c{mqtt}{port} = $broker->{port} + 0;
            $c{mqtt}{username} = $broker->{user};
            $c{mqtt}{password} = encrypt_secret($broker->{pass});
        }
    } elsif (length($c{mqtt}{password} // '')) {
        $c{mqtt}{password} = encrypt_secret($c{mqtt}{password});
    } else {
        $c{mqtt}{password} = $existing->{mqtt}{password} // '';
    }

    open(my $fh, '>', $cfgfile) or err("Konnte config.json nicht schreiben: $!");
    print $fh JSON::PP->new->utf8->pretty->canonical->encode(\%c);
    close $fh;
    return \%c;
}

sub svc_active {
    my ($name) = @_;
    my (undef, $out) = run_capture('systemctl', 'is-active', $name);
    chomp $out;
    return $out;
}

sub svc_status {
    my ($name) = @_;
    my $active = svc_active($name);
    my (undef, $pid) = run_capture('systemctl', 'show', '--property', 'MainPID', '--value', $name);
    chomp $pid;
    undef $pid if !$pid || $pid eq '0' || $active ne 'active';
    return { running => ($active eq 'active') ? JSON::PP::true : JSON::PP::false, pid => $pid, status => $active };
}

sub tail_log {
    my ($file, $n) = @_;
    return '' unless -f $file;
    # :encoding(UTF-8) nötig, sonst doppel-kodiert JSON::PP's ->utf8-Modus
    # bereits rohe UTF-8-Bytes nochmal - siehe KNXtoLOX/api.cgi für die
    # Begründung (dort auf echter Hardware als kaputte Umlaute aufgefallen).
    open(my $fh, '<:encoding(UTF-8)', $file) or return '';
    my @lines = <$fh>;
    close $fh;
    my $start = @lines > $n ? @lines - $n : 0;
    return join('', @lines[$start .. $#lines]);
}

# -- Easee REST: Login-Test + Charger-Liste (rein für die Web-UI, unabhängig
# vom laufenden Go-Daemon) ---------------------------------------------------
# curl statt LWP::UserAgent: gleiche Werkzeugwahl wie KNXtoLOX/api.cgi (dort
# für lox_test/knxproj_import bereits verifiziert verfügbar) - kein weiteres
# Perl-Modul, dessen Verfügbarkeit auf dem Zielsystem ungeklärt wäre.
sub curl_json_post {
    my ($url, $body_hashref, $bearer) = @_;
    my $bodyfile = "/tmp/easeemqtt_post_$$.json";
    open(my $fh, '>', $bodyfile) or return (1, '');
    print $fh JSON::PP->new->utf8->encode($body_hashref // {});
    close $fh;
    my @cmd = ('curl', '-fsS', '--max-time', '10', '-X', 'POST',
               '-H', 'Content-Type: application/json');
    push @cmd, '-H', "Authorization: Bearer $bearer" if $bearer;
    push @cmd, '--data', "\@$bodyfile", $url;
    my ($rc, $out) = run_capture(@cmd);
    unlink $bodyfile;
    return ($rc, $out);
}

sub curl_json_get {
    my ($url, $bearer) = @_;
    my @cmd = ('curl', '-fsS', '--max-time', '10');
    push @cmd, '-H', "Authorization: Bearer $bearer" if $bearer;
    push @cmd, $url;
    return run_capture(@cmd);
}

# Login gegen die echte Easee-Cloud-API. Pfad/Feldnamen (POST /accounts/login,
# {userName,password} -> {accessToken,...}) verifiziert gegen
# developer.easee.com/reference/account_authenticate - siehe Plan-Datei.
sub easee_login {
    my ($username, $password) = @_;
    return (0, undef, 'Benutzername/Passwort fehlt') unless $username && $password;
    my ($rc, $out) = curl_json_post('https://api.easee.com/api/accounts/login',
        { userName => $username, password => $password });
    return (0, undef, 'Easee-Login fehlgeschlagen (Netzwerk/Zugangsdaten)') if $rc != 0;
    my $data = eval { JSON::PP->new->utf8->decode($out) };
    return (0, undef, 'Easee-Antwort nicht lesbar') unless $data && $data->{accessToken};
    return (1, $data->{accessToken}, undef);
}

# Nach einem erfolgreichen Login ist die Geräteliste des Kontos bereits
# bekannt (GET /chargers) - es braucht dafür keinen zweiten, vom Nutzer
# separat ausgelösten Schritt. Deshalb gemeinsam von easee_test (liefert die
# Liste direkt im selben Response mit) UND easee_chargers (expliziter
# Neu-Laden-Aufruf, gleicher Token-mit-jedem-Aufruf-neu-holen-Ansatz, kein
# Caching) genutzt - ein Ort für das id/name-Mapping.
sub fetch_easee_chargers {
    my ($token) = @_;
    my ($rc, $out) = curl_json_get('https://api.easee.com/api/chargers', $token);
    return (0, undef, 'Ladegeräte konnten nicht abgerufen werden') if $rc != 0;
    my $list = eval { JSON::PP->new->utf8->decode($out) };
    return (0, undef, 'Antwort der Easee-API nicht lesbar') unless $list;

    # GET /chargers liefert je nach Kontostruktur unterschiedlich verschachtelte
    # Felder (u.a. je nach Berechtigungsrolle) - hier bewusst tolerant auf
    # id/name gemappt, unbekannte/fehlende Felder werden nicht als Fehler
    # behandelt. Nicht unabhängig gegen ein echtes Mehrfach-Charger-Konto
    # verifiziert (siehe Plan-Datei) - id ist der einzige zwingend benötigte
    # Wert (die eigentliche Charger-ID für alle weiteren API-Aufrufe).
    my @out;
    for my $item (@$list) {
        my $id = $item->{id} // $item->{chargerId};
        next unless $id;
        push @out, { id => $id, name => $item->{name} || $id };
    }
    return (1, \@out, undef);
}

# -- Dispatch ------------------------------------------------------------
my $action = $cgi->url_param('action') // '';
my $method = $ENV{REQUEST_METHOD} // 'GET';

applog("Request: action=$action method=$method");

# Rekursiv Passwörter/Tokens durch "***" ersetzen, BEVOR irgendetwas geloggt
# wird - das Server-Log (api.cgi) ist über die Debug-Seite für jeden mit
# Zugriff auf die LoxBerry-Web-UI im Klartext einsehbar. Betroffene Bodies:
# action=config (easee.password, mqtt.password) UND action=easee_test/
# easee_chargers (username/password werden dort direkt im Body mitgeschickt,
# siehe deren Handler weiter unten). Nur fürs Logging kopiert, die echten
# (unredigierten) Daten fliessen unverändert in die eigentliche Verarbeitung.
sub redact_for_log {
    my ($data) = @_;
    if (ref($data) eq 'HASH') {
        my %out;
        for my $k (keys %$data) {
            $out{$k} = ($k =~ /pass|secret|token/i) ? '***' : redact_for_log($data->{$k});
        }
        return \%out;
    }
    if (ref($data) eq 'ARRAY') {
        return [ map { redact_for_log($_) } @$data ];
    }
    return $data;
}

sub read_json_body {
    my $raw = $cgi->param('POSTDATA');
    if (!defined($raw) || $raw eq '') {
        local $/;
        $raw = <STDIN>;
    }
    return {} unless $raw;
    my $data = eval { JSON::PP->new->utf8->decode($raw) };
    if (!$data) {
        applog("JSON-Parse-Fehler im Request-Body (" . length($raw) . " Bytes): $@");
        return {};
    }
    applog("Body: " . JSON::PP->new->utf8->canonical->encode(redact_for_log($data)));
    return $data || {};
}

if ($action eq 'config') {
    if ($method eq 'POST') {
        my $body = read_json_body();
        my $cfg = save_config($body);
        run_capture('sudo', 'systemctl', 'restart', $SERVICE);
        applog('Konfiguration gespeichert, Dienst neu gestartet');
        out({ ok => JSON::PP::true });
    } else {
        my $cfg = load_config();
        my $broker = mqtt_broker_conn();
        $cfg->{local_broker_info} = $broker ? "$broker->{host}:$broker->{port}" : '';
        # Passwoerter (verschluesselt gespeichert) NIE an den Browser
        # zurueckgeben - nur ob ueberhaupt eines gesetzt ist, damit das
        # Frontend einen passenden Platzhalter zeigen kann. Leeres Feld beim
        # naechsten Speichern behaelt den bestehenden Wert (siehe
        # save_config()).
        $cfg->{easee}{password_set} = length($cfg->{easee}{password} // '') ? JSON::PP::true : JSON::PP::false;
        $cfg->{mqtt}{password_set}  = length($cfg->{mqtt}{password}  // '') ? JSON::PP::true : JSON::PP::false;
        $cfg->{easee}{password} = '';
        $cfg->{mqtt}{password}  = '';
        out($cfg);
    }
}
elsif ($action eq 'easee_test') {
    # Login-Test UND Geräteliste in einem Aufruf: nach einem erfolgreichen
    # Login kennt Easee bereits alle Charger des Kontos (GET /chargers) - ein
    # zweiter, vom Nutzer separat ausgelöster "Ladegeräte suchen"-Schritt
    # wäre reine Redundanz (derselbe Login wäre sonst zweimal nötig).
    my $body = read_json_body();
    my $cfg = load_config();
    my $username = length($body->{username} // '') ? $body->{username} : $cfg->{easee}{username};
    my $password = length($body->{password} // '') ? $body->{password} : decrypt_secret($cfg->{easee}{password});
    my ($ok, $token, $error) = easee_login($username, $password);
    out({ ok => JSON::PP::false, error => $error }) unless $ok;

    my ($cok, $chargers, $cerror) = fetch_easee_chargers($token);
    # Login war erfolgreich, auch wenn das Nachladen der Geräteliste
    # scheitert (z.B. vorübergehender API-Fehler) - deshalb weiterhin ok=true,
    # chargers bleibt dann leer und chargers_error erklärt warum.
    out({ ok => JSON::PP::true, chargers => $cok ? $chargers : [], chargers_error => $cok ? undef : $cerror });
}
elsif ($action eq 'easee_chargers') {
    # Manuelles Neu-Laden der Geräteliste (z.B. wenn seither ein neuer
    # Charger im Easee-Konto hinzugekommen ist), ohne den Login-Test-Text
    # erneut anzuzeigen. Nutzt bevorzugt die gerade im Formular getippten
    # Zugangsdaten (Body), fallback auf die bereits gespeicherten - so
    # funktioniert der Button auch VOR dem ersten "Speichern".
    my $body = read_json_body();
    my $cfg = load_config();
    my $username = length($body->{username} // '') ? $body->{username} : $cfg->{easee}{username};
    my $password = length($body->{password} // '') ? $body->{password} : decrypt_secret($cfg->{easee}{password});
    my ($ok, $token, $error) = easee_login($username, $password);
    err($error || 'Easee-Login fehlgeschlagen') unless $ok;

    my ($cok, $chargers, $cerror) = fetch_easee_chargers($token);
    err($cerror || 'Ladegeräte konnten nicht abgerufen werden') unless $cok;
    out({ chargers => $chargers });
}
elsif ($action eq 'service_status') {
    my $s = svc_status($SERVICE);
    $s->{log} = tail_log($enginelogfile, 150);
    out($s);
}
elsif ($action eq 'service_restart') {
    applog("Neustart von $SERVICE angefordert");
    my (undef, $out) = run_capture('sudo', 'systemctl', 'restart', $SERVICE);
    out({ ok => JSON::PP::true, output => $out });
}
elsif ($action eq 'api_log') {
    out({ log => tail_log("$lbplogdir/api.log", 150) });
}
else {
    err('Unbekannte Aktion');
}
