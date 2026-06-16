package handlers

import "testing"

func TestParseIntPathID(t *testing.T) {
	tests := []struct {
		path    string
		prefix  string
		want    int64
		wantErr bool
	}{
		{"/api/expenses/42", "/api/expenses/", 42, false},
		{"/api/expenses/1", "/api/expenses/", 1, false},
		{"/api/expenses/", "/api/expenses/", 0, true},   // empty suffix
		{"/api/expenses/0", "/api/expenses/", 0, true},   // zero is invalid
		{"/api/expenses/-1", "/api/expenses/", 0, true},  // negative
		{"/api/expenses/abc", "/api/expenses/", 0, true}, // non-numeric
	}
	for _, tc := range tests {
		got, err := parseIntPathID(tc.path, tc.prefix)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseIntPathID(%q, %q) error = %v, wantErr %v", tc.path, tc.prefix, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("parseIntPathID(%q, %q) = %d, want %d", tc.path, tc.prefix, got, tc.want)
		}
	}
}

func TestParseSplitParticipantIds(t *testing.T) {
	tests := []struct {
		input string
		want  []int64
	}{
		{"", []int64{}},
		{"1", []int64{1}},
		{"1,2,3", []int64{1, 2, 3}},
		{"1, 2, 3", []int64{1, 2, 3}},  // spaces trimmed
		{"-1,0,2", []int64{2}},          // negative and zero skipped
		{"abc,2", []int64{2}},           // non-numeric skipped
	}
	for _, tc := range tests {
		got := parseSplitParticipantIds(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("parseSplitParticipantIds(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseSplitParticipantIds(%q)[%d] = %d, want %d", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestSplitAmount(t *testing.T) {
	// splitAmount(amount, percentage) = math.Round(amount*percentage) / 100
	tests := []struct {
		amount     float64
		percentage float64
		want       float64
	}{
		{100, 50, 50},
		{100, 33.333, 33.33},   // math.Round(3333.3)/100 = 3333/100 = 33.33
		{100, 66.667, 66.67},   // math.Round(6666.7)/100 = 6667/100 = 66.67
		{0, 50, 0},
		{200, 25, 50},
	}
	for _, tc := range tests {
		got := splitAmount(tc.amount, tc.percentage)
		if got != tc.want {
			t.Errorf("splitAmount(%v, %v) = %v, want %v", tc.amount, tc.percentage, got, tc.want)
		}
	}
}
