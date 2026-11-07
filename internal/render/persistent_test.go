package render

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rigerc/smol/internal/icons"
	"github.com/rigerc/smol/internal/modules"
)

func TestAssembleFaceLine(t *testing.T) {
	toks, err := Parse("λ {missing} · {face} / {host} + {face} !")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]modules.Result{"face": {Text: "(x)"}, "host": {Text: "界"}}
	opts := Options{Shell: ShellNone, Color: true, ShowIcons: true, Icons: icons.Options{Mode: "always", Set: "ascii", Overrides: map[string]string{"face": "FACE"}}, Styles: map[string]Style{"face": {FG: "cyan"}}}
	line := AssembleFaceLine(toks, values, opts, 6)
	want := FaceLine{Text: "λ \x1b[36mFACE      \x1b[0m / HOST界 + \x1b[36mFACE      \x1b[0m !", Columns: 36, Faces: []int{6, 28}}
	if !reflect.DeepEqual(line, want) {
		t.Fatalf("wrong layout: got %+v; want %+v", line, want)
	}
	plain := opts
	plain.Color = false
	flat := AssembleFaceLine(toks, values, plain, 6)
	if line.Columns != flat.Columns || line.Faces[1]+6+2 != line.Columns {
		t.Fatalf("inconsistent width: %+v", line)
	}
	original := Assemble(toks, values, opts)
	if !strings.Contains(original, "(x)") || original == line.Text {
		t.Fatal("static rendering changed")
	}
}
