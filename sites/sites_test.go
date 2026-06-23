package sites

import (
	"testing"
)

func TestSite_ThemeValue(t *testing.T) {
	cases := []struct {
		name string
		site *Site
		want string
	}{
		{"nil site", nil, "light"},
		{"empty theme", &Site{Theme: ""}, "light"},
		{"set theme", &Site{Theme: "dark"}, "dark"},
		{"custom theme", &Site{Theme: "cupcake"}, "cupcake"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.site.ThemeValue(); got != tc.want {
				t.Errorf("ThemeValue() = %q, want %q", got, tc.want)
			}
		})
	}
}
