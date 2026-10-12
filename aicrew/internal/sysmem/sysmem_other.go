//go:build !windows

package sysmem

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func read() (Status, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return Status{}, err
	}
	defer f.Close()
	var s Status
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		kb, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			s.Total = kb << 10
		case "MemAvailable:":
			s.Available = kb << 10
		}
	}
	return s, sc.Err()
}
