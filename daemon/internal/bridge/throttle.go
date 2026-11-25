package bridge

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// parseAmps liest einen Dynamic-Current-Sollwert und rundet ihn auf ganze
// Ampere AB. Hintergrund: die Easee-API erwartet "amps" als Ganzzahl
// (developer.easee.com, charger_set_dynamic_charger_current), und Loxones
// Ueberschussregelung liefert Bruchteile, die sich fast sekuendlich aendern.
// Abrunden statt kaufmaennisch runden: ein Sollwert aus Lastmanagement oder
// PV-Ueberschuss ist eine Obergrenze, 13,7 A verfuegbar heisst 13 A laden.
// Negative Werte werden zu 0.
func parseAmps(payload string) (int, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(payload), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		if err == nil {
			err = strconv.ErrSyntax
		}
		return 0, err
	}
	a := int(math.Floor(f + 1e-6)) // 5.9999999 aus Gleitkommarauschen -> 6
	if a < 0 {
		a = 0
	}
	return a, nil
}

// ampThrottle begrenzt, wie oft set/dynamic_current an die Cloud geht.
// Regeln (pro Topic):
//   - Erhoehungen frühestens minInterval nach dem letzten Call; was in der
//     Zwischenzeit kommt, wird gesammelt und der LETZTE Wert nach Ablauf
//     nachgesendet.
//   - Sofort: der erste Wert, 0 (Laden stoppen), jede Absenkung (Lastmanagement:
//     eine Ueberlast darf nicht 30 s warten) und ein unveraenderter Wert
//     (den filtert danach sendGate, ggf. gewollt erneut nach Wallbox-Neustart).
//   - minInterval <= 0 schaltet die Drosselung ab.
type ampThrottle struct {
	mu          sync.Mutex
	minInterval time.Duration
	state       map[string]*throttleState
}

type throttleState struct {
	has     bool
	lastAt  time.Time
	lastVal int
	pending int
	timer   *time.Timer
	gen     int // verhindert, dass ein bereits feuernder Timer eine Absenkung ueberholt
}

func newAmpThrottle(minInterval time.Duration) *ampThrottle {
	return &ampThrottle{minInterval: minInterval, state: map[string]*throttleState{}}
}

func (t *ampThrottle) get(key string) *throttleState {
	s := t.state[key]
	if s == nil {
		s = &throttleState{}
		t.state[key] = s
	}
	return s
}

// delay: 0 = sofort senden, sonst Wartezeit bis zum naechsten erlaubten Call.
func (t *ampThrottle) delay(s *throttleState, v int, now time.Time) time.Duration {
	if t.minInterval <= 0 || !s.has || v == 0 || v <= s.lastVal {
		return 0
	}
	if d := s.lastAt.Add(t.minInterval).Sub(now); d > 0 {
		return d
	}
	return 0
}

// submit ruft send(v) sofort auf oder plant den zuletzt uebergebenen Wert fuer
// spaeter ein. send() legt nur in die Charger-Warteschlange (blockiert nicht).
func (t *ampThrottle) submit(key string, v int, now time.Time, send func(int)) {
	t.mu.Lock()
	s := t.get(key)
	d := t.delay(s, v, now)
	if d <= 0 {
		if s.timer != nil {
			s.timer.Stop()
			s.timer = nil
		}
		s.gen++
		t.mu.Unlock()
		send(v)
		return
	}
	s.pending = v
	if s.timer == nil {
		gen := s.gen
		s.timer = time.AfterFunc(d, func() {
			t.mu.Lock()
			if s.gen != gen {
				t.mu.Unlock()
				return
			}
			s.timer = nil
			p := s.pending
			t.mu.Unlock()
			send(p)
		})
	}
	t.mu.Unlock()
}

// sent: der Call fuer v wurde (versucht) abgesetzt - Bezugspunkt fuer
// Mindestabstand und "ist das eine Absenkung?". Bewusst auch bei Fehlschlag,
// sonst wuerde z.B. ein Rate-Limit-Fehler sofortige Wiederholungen ausloesen.
func (t *ampThrottle) sent(key string, v int, now time.Time) {
	t.mu.Lock()
	s := t.get(key)
	s.has, s.lastAt, s.lastVal = true, now, v
	t.mu.Unlock()
}
