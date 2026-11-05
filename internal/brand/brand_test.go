package brand

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSharedArtwork(t *testing.T) {
	art, err := Art()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(art, "|___/_| |_| |_|\\___/|_|") {
		t.Fatalf("not Standard lettering: %q", art)
	}
	for _, light := range []bool{false, true} {
		colored := Gradient(art, light)
		if ansi.Strip(colored) != art {
			t.Fatal("gradient changed lettering")
		}
	}
	if LightBackground("") || LightBackground("bad") || LightBackground("15;0") || !LightBackground("0;15") || !LightBackground("0;7") {
		t.Fatal("background hints")
	}
	stops := Stops(false)
	stops[0] = "changed"
	if Stops(false)[0] != "#F83600" {
		t.Fatal("mutable palette")
	}
}
