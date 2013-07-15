package ndb

import (
	"testing"
)

var structWriteTests = []struct {
	in  netCfg
	out string
}{
	{
		netCfg{"p2-jbs239", []int{64, 52, 100}, 666},
		"host-name=p2-jbs239 vlan=64 vlan=52 vlan=100 native-vlan=666",
	},
	{
		netCfg{"p2-cass304", []int{55, 10}, 1},
		"host-name=p2-cass304 vlan=55 vlan=10 native-vlan=1",
	},
}

var mapWriteTests = []struct {
	in map[string] string
	out string
}{
	{
		map[string] string {"user": "jenkins", "group": "jenkins"},
		"user=jenkins group=jenkins",
	},
}

func TestStructWrite(t *testing.T) {
	for _, tt := range structWriteTests {
		if b, err := Emit(tt.in); err != nil {
			t.Error(err)
		} else if string(b) != tt.out {
			t.Errorf("Wanted %s, got %s", tt.out, string(b))
		} else {
			t.Logf("%v => %s", tt.in, string(b))
		}
	}
}

func TestMapWrite(t *testing.T) {
	for _, tt := range mapWriteTests {
		if b, err := Emit(tt.in); err != nil {
			t.Error(err)
		} else if string(b) != tt.out {
			t.Errorf("Wanted %s, got %s", tt.out, string(b))
		} else {
			t.Logf("%v => %s", tt.in, string(b))
		}
	}
}
