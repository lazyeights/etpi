package etpi

import "testing"

func TestNewCommandFromBytes(t *testing.T) {
	p := []byte{53, 48, 53, 51, 67, 68, 13, 10}
	cmd, err := NewCommandFromBytes(p)
	if err != nil {
		t.Error(err)
	}
	if cmd.Code != "505" || cmd.Data[0] != '3' {
		t.Errorf("expected 5053, got %s%s", cmd.Code, string(cmd.Data[0]))
	}

	p = []byte{53, 48, 53, 51, 67, 67, 13, 10}
	cmd, err = NewCommandFromBytes(p)
	if err == nil {
		t.Error("expected bad checksum")
	}
}
