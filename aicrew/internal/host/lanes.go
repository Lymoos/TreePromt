package host

import (
	"sync"

	"prompttree/aicrew/internal/sysmem"
)

// Очереди хоста (этап 7.8, docs/stage7-8-codex.md): основная (Claude, Junior, слияние, планы) и очередь
// Codex идут параллельно. Вторая очередь берёт задачу, только когда это безопасно для ПК.

const (
	LaneMain  = "main"
	LaneCodex = "codex"
)

// Lanes — что сейчас выполняет каждая очередь (исполнитель задачи или пусто).
type Lanes struct {
	mu      sync.Mutex
	running map[string]string
	// MinFreeRAM — сколько памяти должно быть свободно, чтобы вторая очередь начала задачу (0 — не проверять).
	MinFreeRAM uint64
	// Memory — nil означает sysmem.Read.
	Memory func() (sysmem.Status, error)
}

func (l *Lanes) set(lane, executor string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running == nil {
		l.running = map[string]string{}
	}
	l.running[lane] = executor
}

func (l *Lanes) get(lane string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running[lane]
}

// mayStart — можно ли очереди lane брать новую задачу. Основная берёт всегда (защиту Junior делает сам
// исполнитель); остальные ждут, пока основная гоняет Junior (ему нужна вся видеокарта и часть памяти)
// и пока свободной памяти меньше MinFreeRAM (два контейнера с Flutter тяжёлые; 12.10.2026 был синий экран).
func (l *Lanes) mayStart(lane string) bool {
	if l == nil || lane == "" || lane == LaneMain {
		return true
	}
	if l.get(LaneMain) == "junior" {
		return false
	}
	if l.MinFreeRAM == 0 {
		return true
	}
	read := l.Memory
	if read == nil {
		read = sysmem.Read
	}
	st, err := read()
	return err != nil || st.Available >= l.MinFreeRAM
}
