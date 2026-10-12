// Package sysmem — сколько памяти свободно на ПК: Junior не запускается, если локальная модель
// не влезает с запасом (12.10.2026 модель 30B вместе с Docker довела ПК до синего экрана).
package sysmem

// Status — физическая память, байты.
type Status struct {
	Total     uint64
	Available uint64
}

// Read — текущее состояние памяти.
func Read() (Status, error) { return read() }
