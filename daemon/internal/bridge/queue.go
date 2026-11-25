package bridge

import "sync"

// job ist eine auszufuehrende Aktion (REST-Call) fuer einen Charger.
// key != "": zusammenfassbar - ein noch wartender Job mit demselben key wird
// durch den neueren ersetzt ("letzter Wert gewinnt", fuer set/*-Topics).
// key == "": nie zusammenfassen (cmd/*: zwei toggle sind zwei toggle).
type job struct {
	key string
	run func()
}

// chargerQueue arbeitet die Jobs EINES Chargers nacheinander in einer eigenen
// Goroutine ab. Hintergrund: paho ruft Message-Handler (OrderMatters=true,
// Standard) nacheinander auf - lief der REST-Call (bis 15 s Timeout) direkt
// im Handler, blockierte ein haengender Cloud-Call ALLE anderen Kommandos
// aller Charger. Die Handler legen jetzt nur noch ab; die Reihenfolge pro
// Charger bleibt erhalten, verschiedene Charger blockieren sich nicht.
type chargerQueue struct {
	mu      sync.Mutex
	items   []*job
	pending map[string]*job
	wake    chan struct{}
}

func newChargerQueue() *chargerQueue {
	return &chargerQueue{pending: map[string]*job{}, wake: make(chan struct{}, 1)}
}

func (q *chargerQueue) push(j *job) {
	q.mu.Lock()
	if j.key != "" {
		if p, ok := q.pending[j.key]; ok {
			p.run = j.run // Position in der Warteschlange bleibt, Wert wird aktualisiert
			q.mu.Unlock()
			return
		}
		q.pending[j.key] = j
	}
	q.items = append(q.items, j)
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *chargerQueue) next() *job {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	j := q.items[0]
	q.items = q.items[1:]
	if j.key != "" {
		delete(q.pending, j.key)
	}
	return j
}

// loop laeuft, bis done geschlossen wird.
func (q *chargerQueue) loop(done <-chan struct{}) {
	for {
		for j := q.next(); j != nil; j = q.next() {
			j.run()
		}
		select {
		case <-done:
			return
		case <-q.wake:
		}
	}
}
