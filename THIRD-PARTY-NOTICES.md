# Third-Party Notices

Der `easeemqtt`-Daemon (`daemon/`) bindet folgende Open-Source-Bibliotheken
statisch ein. Diese Datei listet die direkten Abhängigkeiten aus
`daemon/go.mod` mitsamt Lizenz; der jeweilige Lizenztext ist beim
Originalprojekt verlinkt.

| Modul | Version | Lizenz | Projekt |
|---|---|---|---|
| github.com/eclipse/paho.mqtt.golang | v1.5.1 | EPL-2.0 / EDL-1.0 (dual, Eclipse Foundation) | https://github.com/eclipse/paho.mqtt.golang |
| github.com/philippseith/signalr | v0.11.0 | MIT | https://github.com/philippseith/signalr |
| github.com/cenkalti/backoff/v4 | v4.3.0 | MIT | https://github.com/cenkalti/backoff |

Alle drei Lizenzen erlauben uneingeschränkte Nutzung/Weitergabe (auch in
kompilierter Binärform), solange Copyright-Hinweis und Lizenztext erhalten
bleiben - das übernimmt diese Datei stellvertretend für alle drei.

## Hinweis zu transitiven Abhängigkeiten

Diese Tabelle deckt nur die drei DIREKTEN Abhängigkeiten aus `go.mod` ab.
Da `go.sum` bewusst nicht im Repo committet ist (wird bei jeder Installation
frisch per `go mod tidy` aufgelöst, siehe `postroot.sh`), lässt sich die
vollständige transitive Abhängigkeitskette hier nicht statisch auflisten.
Bei Bedarf lokal nachvollziehbar mit:

```
cd daemon && go mod tidy && go list -m all
```

oder für eine lizenzspezifische Aufstellung mit dem Tool
[`google/go-licenses`](https://github.com/google/go-licenses):

```
go install github.com/google/go-licenses@latest
cd daemon && go-licenses report ./cmd/easeemqtt
```

## Perl-Frontend (`webfrontend/htmlauth/`)

Nutzt ausschliesslich LoxBerry-eigene Perl-Module (`LoxBerry::System`,
`LoxBerry::Web`) sowie Perl-Core-/Standard-Module (`CGI`, `JSON::PP`,
`strict`, `warnings`) - keine zusaetzlichen Drittanbieter-Abhaengigkeiten,
daher hier nicht gesondert aufgefuehrt.
