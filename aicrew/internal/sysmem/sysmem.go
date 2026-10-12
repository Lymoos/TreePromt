// Package sysmem — сколько памяти свободно на ПК: Junior не запускается, если локальная модель
// не влезает с запасом (12.10.2026 модель 30B вместе с Docker довела ПК до синего экрана).
package sysmem

import (
	"os/exec"
	"strconv"
	"strings"
)

// Status — физическая память, байты.
type Status struct {
	Total     uint64
	Available uint64
}

// Read — текущее состояние памяти.
func Read() (Status, error) { return read() }

// GPUFree — свободная память видеокарты NVIDIA (nvidia-smi), байты; 0 — карты нет или не узнали.
// Ollama кладёт модель сначала в видеопамять, в оперативную уходит только остаток.
func GPUFree() uint64 {
	out, err := exec.Command("nvidia-smi", "--query-gpu=memory.free", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return 0
	}
	var total uint64
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		mib, err := strconv.ParseUint(strings.TrimSpace(line), 10, 64)
		if err == nil {
			total += mib << 20
		}
	}
	return total
}
