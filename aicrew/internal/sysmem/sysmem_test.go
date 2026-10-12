package sysmem

import "testing"

func TestReadReportsRealMemory(t *testing.T) {
	s, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if s.Total < 1<<30 || s.Available == 0 || s.Available > s.Total {
		t.Fatalf("%+v", s)
	}
}
