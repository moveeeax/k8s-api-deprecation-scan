package scan

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in    string
		major int
		minor int
	}{
		{"1.29", 1, 29},
		{"v1.29", 1, 29},
		{"1.29.3", 1, 29},
		{" 1.9 ", 1, 9},
	}
	for _, c := range cases {
		got, err := ParseVersion(c.in)
		if err != nil {
			t.Errorf("ParseVersion(%q): %v", c.in, err)
			continue
		}
		if got.Major != c.major || got.Minor != c.minor {
			t.Errorf("ParseVersion(%q) = %v, want %d.%d", c.in, got, c.major, c.minor)
		}
	}
}

func TestParseVersionRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "1", "1.x", "x.1", "1.2.3.4", "latest", "1.2.x"} {
		if v, err := ParseVersion(in); err == nil {
			t.Errorf("ParseVersion(%q) = %v, want an error", in, v)
		}
	}
}

func TestAtLeast(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.29", "1.29", true},
		{"1.30", "1.29", true},
		{"1.28", "1.29", false},
		{"1.9", "1.16", false},
		{"1.16", "1.9", true},
		{"2.0", "1.99", true},
	}
	for _, c := range cases {
		a, _ := ParseVersion(c.a)
		b, _ := ParseVersion(c.b)
		if got := a.AtLeast(b); got != c.want {
			t.Errorf("%s.AtLeast(%s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestVersionString(t *testing.T) {
	v, _ := ParseVersion("v1.29.4")
	if v.String() != "1.29" {
		t.Errorf("String() = %q, want 1.29", v.String())
	}
}
