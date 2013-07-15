package ndb

import (
	"bytes"
	"fmt"
	"testing"
)

type screenCfg struct {
	Title         string
	Width, Height uint16
	R, G, B, A    uint16
}

type netCfg struct {
	Host   string `ndb:"host-name"`
	Vlan   []int  `ndb:"vlan"`
	Native int    `ndb:"native-vlan"`
}

var multiMap = []struct {
	in  string
	out map[string][]string
}{
	{
		in: "user=clive user=david user=trenton group=dirty-dozen",
		out: map[string][]string{
			"user":  []string{"clive", "david", "trenton"},
			"group": []string{"dirty-dozen"},
		},
	},
}

var advancedTests = []struct {
	in  string
	out netCfg
}{
	{in: "host-name=p2-jbs537 vlan=66 vlan=35 native-vlan=218",
		out: netCfg{
			Host:   "p2-jbs537",
			Vlan:   []int{66, 35},
			Native: 218,
		},
	},
}

var structTests = []struct {
	in  string
	out screenCfg
}{
	{
		in: "Title='Hollywood movie' Width=640 Height=400 A=8",
		out: screenCfg{
			Title:  "Hollywood movie",
			Width:  640,
			Height: 400,
			A:      8,
		},
	},
}

var mapTests = []struct {
	in  string
	out map[string]string
}{
	{
		in: "ipnet=murray-hill ip=135.104.0.0 ipmask=255.255.0.0",
		out: map[string]string{
			"ipnet":  "murray-hill",
			"ip":     "135.104.0.0",
			"ipmask": "255.255.0.0",
		},
	},
}

func TestStruct(t *testing.T) {
	var cfg screenCfg

	for _, tt := range structTests {
		if err := Unmarshal([]byte(tt.in), &cfg); err != nil {
			t.Error(err)
		} else if cfg != tt.out {
			t.Errorf("Got %v, wanted %v", cfg, tt.out)
		}
		t.Logf("%s == %v", tt.in, cfg)
	}
}
func TestMap(t *testing.T) {
	var net map[string]string
	for _, tt := range mapTests {
		if err := Unmarshal([]byte(tt.in), &net); err != nil {
			t.Error(err)
		} else if !mapEquals(tt.out, net) {
			t.Errorf("Got `%v`, wanted `%v`", net, tt.out)
		}
		t.Logf("%s == %v", tt.in, net)
	}
}

func mapEquals(m1, m2 map[string] string) bool {
	for k := range m1 {
		if m1[k] != m2[k] {
			return false
		}
	}
	for k := range m2 {
		if m1[k] != m2[k] {
			return false
		}
	}
	return true
}

func TestAdvanced(t *testing.T) {
	var net netCfg
	for _, tt := range advancedTests {
		if err := Unmarshal([]byte(tt.in), &net); err != nil {
			t.Error(err)
		} else if fmt.Sprint(tt.out) != fmt.Sprint(net) {
			t.Errorf("Got %v, wanted %v", net, tt.out)
		}
		t.Logf("%s == %v", tt.in, net)
	}
}

func TestMultiMap(t *testing.T) {
	var m map[string][]string
	for _, tt := range multiMap {
		if err := Unmarshal([]byte(tt.in), &m); err != nil {
			t.Error(err)
		} else if fmt.Sprint(tt.out) != fmt.Sprint(m) {
			t.Errorf("Got %v, wanted %v", m, tt.out)
		}
		t.Logf("%s == %v", tt.in, m)
	}
}

var parseTests = []struct {
	in  []byte
	out []pair
}{
	{
		in: []byte("key1=val1 key2=val2 key3=val3"),
		out: []pair{
			{[]byte("key1"), []byte("val1")},
			{[]byte("key2"), []byte("val2")},
			{[]byte("key3"), []byte("val3")}},
	},
	{
		in: []byte("title='Some value with spaces' width=340 height=200"),
		out: []pair{
			{[]byte("title"), []byte("Some value with spaces")},
			{[]byte("width"), []byte("340")},
			{[]byte("height"), []byte("200")}},
	},
	{
		in: []byte("title='Dave''s pasta' sq=Davis cost=$$"),
		out: []pair{
			{[]byte("title"), []byte("Dave's pasta")},
			{[]byte("sq"), []byte("Davis")},
			{[]byte("cost"), []byte("$$")}},
	},
	{
		in: []byte("action=''bradley key=jay mod=ctrl+alt+shift"),
		out: []pair{
			{[]byte("action"), []byte("'bradley")},
			{[]byte("key"), []byte("jay")},
			{[]byte("mod"), []byte("ctrl+alt+shift")}},
	},
	{
		in: []byte("action=reload key='' mod=ctrl+alt+shift"),
		out: []pair{
			{[]byte("action"), []byte("reload")},
			{[]byte("key"), []byte("'")},
			{[]byte("mod"), []byte("ctrl+alt+shift")}},
	},
	{
		in: []byte("s='spaces and '' quotes'"),
		out: []pair{
			{[]byte("s"), []byte("spaces and ' quotes")}},
	},
	{
		in: []byte("esc='Use '''' to escape a '''"),
		out: []pair{
			{[]byte("esc"), []byte("Use '' to escape a '")}},
	},
}

func Test_parsing(t *testing.T) {
	for i, tt := range parseTests {
		d := NewDecoder(bytes.NewReader(tt.in))
		p, err := d.getPairs()
		if err != nil {
			t.Error(err)
			t.FailNow()
		} else {
			for j := range tt.out {
				if j > len(p) || !match(p[j], tt.out[j]) {
					t.Errorf("%d: getPairs %s => %v, want %v", i, tt.in, p, tt.out)
					t.FailNow()
				}
			}
		}
	}
}

func match(p1, p2 pair) bool {
	return (bytes.Compare(p1.attr, p2.attr) == 0) && (bytes.Compare(p1.val, p2.val) == 0)
}
