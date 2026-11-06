package setup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/face"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/tuiweave"
	"github.com/rigerc/tuiweave/inspect"
	"github.com/rigerc/tuiweave/snaptest"
	"github.com/rigerc/tuiweave/spinner"
)

func testRuntime(t *testing.T, input, path string) Runtime {
	t.Helper()
	return Runtime{Input: strings.NewReader(input), CWD: filepath.Dir(path), StdinTTY: true, StderrTTY: true, Basic: true, Width: 80, Rows: 24, Entropy: entropySource(bytes.NewReader(bytes.Repeat([]byte{42}, 128))), Getenv: func(k string) string {
		if k == "HOME" {
			return filepath.Dir(path)
		}
		if k == "TERM" {
			return "dumb"
		}
		return ""
	}}
}
func testEnv() modules.Env {
	return modules.Env{FS: os.DirFS("../../testdata/x86"), Getenv: func(string) string { return "" }, Now: func() time.Time { return time.Unix(0, 0) }}
}
func runSetup(t *testing.T, input, path string) (int, string, string) {
	t.Helper()
	var out, diag bytes.Buffer
	code := Run(context.Background(), []string{"--config", path}, &out, &diag, testRuntime(t, input, path), testEnv())
	return code, out.String(), diag.String()
}
func TestNewConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	code, out, diag := runSetup(t, strings.Repeat("\n", 16)+"2\n", path)
	if code != 0 || out != path+"\n" {
		t.Fatalf("code %d out %q diag %s", code, out, diag)
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is a configuration fixture inside t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(path, data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Format != config.DefaultFormat || cfg.Color != "auto" {
		t.Fatalf("config %#v", cfg)
	}
	stat, err := os.Stat(path)
	if err != nil || stat.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v %v", stat, err)
	}
	stat, err = os.Stat(filepath.Dir(path))
	if err != nil || stat.Mode().Perm() != 0o700 {
		t.Fatalf("parent %v %v", stat, err)
	}
	if !strings.Contains(diag, "Exact TOML") || !strings.Contains(diag, "Saved ") {
		t.Fatal(diag)
	}
}
func TestExistingPreservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := `format = "prefix {{braces}} {host}/{host}"
color = "never"
[display.size]
unit = "si"
digits = 3
[modules.disk]
mount = "/srv"
[style.cpu]
fg = "red"
bold = true
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil { //nolint:gosec // G302: verifies preservation of an existing group-readable config.
		t.Fatal(err)
	}
	code, out, diag := runSetup(t, strings.Repeat("\n", 12)+"2\n", path)
	if code != 0 || out != path+"\n" {
		t.Fatalf("code %d %s", code, diag)
	}
	cfg, err := config.Load(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Format != "prefix {{braces}} {host}/{host}" || cfg.Display.Size.Unit != "si" || cfg.Display.Size.Digits != 3 || cfg.Modules.Disk.Mount != "/srv" || !cfg.Style["cpu"].Bold {
		t.Fatalf("lost config: %#v", cfg)
	}
	stat, _ := os.Stat(path)
	if stat.Mode().Perm() != 0o640 {
		t.Fatal(stat.Mode())
	}
}
func TestCancellationAndDefaultNo(t *testing.T) {
	for _, input := range []string{"", ":cancel\n", "2\n", strings.Repeat("\n", 14)} {
		t.Run(strings.ReplaceAll(input, "\n", "_"), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "new", "config.toml")
			code, out, diag := runSetup(t, input, path)
			if code != 130 || out != "" {
				t.Fatalf("code %d out %q diag %s", code, out, diag)
			}
			if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected writes: %v", err)
			}
		})
	}
}
func TestInvocation(t *testing.T) {
	for _, args := range [][]string{{"--config"}, {"--config="}, {"--config", "a", "--config=b"}, {"--force"}, {"extra"}, {"--help", "--bad"}} {
		if _, err := parse(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, args := range [][]string{{"--help"}, {"--config=a"}, {"--config", "a"}, {"--help", "--config", "a"}} {
		if _, err := parse(args); err != nil {
			t.Fatal(err)
		}
	}
	var out, diag bytes.Buffer
	if code := Run(context.Background(), []string{"--help"}, &out, &diag, Runtime{}, modules.Env{}); code != 0 || out.Len() != 0 {
		t.Fatalf("help %d", code)
	}
	diag.Reset()
	if code := Run(context.Background(), nil, &out, &diag, Runtime{}, modules.Env{}); code != 2 {
		t.Fatalf("tty %d", code)
	}
	runtime := testRuntime(t, "", t.TempDir())
	runtime.CWD = ""
	runtime.Getenv = func(string) string { return "" }
	if code := Run(context.Background(), nil, &out, &diag, runtime, modules.Env{}); code != 2 {
		t.Fatalf("path %d", code)
	}
}
func TestDestination(t *testing.T) {
	for _, tc := range []struct {
		args                 invocation
		cwd, home, xdg, want string
	}{
		{invocation{path: "a.toml"}, "/work", "/home/a", "/xdg", "/work/a.toml"},
		{invocation{}, "/work", "/home/a", "/xdg", "/xdg/smol/config.toml"},
		{invocation{}, "/work", "/home/a", "", "/home/a/.config/smol/config.toml"},
	} {
		got, err := destination(tc.args, tc.cwd, func(k string) string {
			if k == "HOME" {
				return tc.home
			}
			if k == "XDG_CONFIG_HOME" {
				return tc.xdg
			}
			return ""
		})
		if err != nil || got != tc.want {
			t.Fatalf("%q %v", got, err)
		}
	}
	for _, path := range []string{"\x1b", "\n"} {
		if _, err := destination(invocation{path: path}, "/work", func(string) string { return "" }); err == nil {
			t.Fatal("controls accepted")
		}
	}
	if _, err := destination(invocation{path: "relative"}, "", func(string) string { return "" }); err == nil {
		t.Fatal("relative cwd accepted")
	}
}
func freshSession(t *testing.T) *session {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	runtime := testRuntime(t, "", path)
	return &session{term: &terminal{runtime: runtime, out: io.Discard}, env: testEnv(), path: path, draft: config.Default(), fields: map[string]string{}, state: "preset.ready", preset: "default", shell: "none"}
}
func accept(t *testing.T, s *session, value string) {
	t.Helper()
	if err := s.accept(context.Background(), value); err != nil {
		t.Fatal(err)
	}
}
func TestPresetOverlays(t *testing.T) {
	for _, preset := range []string{"default", "minimal", "banner", "prompt", "server", "existing"} {
		t.Run(preset, func(t *testing.T) {
			s := freshSession(t)
			s.preset = "existing"
			s.draft.Color = "always"
			s.draft.Display.Size.Unit = "si"
			s.draft.Modules.Disk.Mount = "/srv"
			style := s.draft.Style["cpu"]
			style.Bold = true
			s.draft.Style["cpu"] = style
			accept(t, s, preset)
			if s.draft.Display.Size.Unit != "si" || !s.draft.Style["cpu"].Bold {
				t.Fatal("overlay reset unrelated values")
			}
			switch preset {
			case "default":
				if s.draft.Color != "auto" {
					t.Fatal(s.draft.Color)
				}
			case "minimal":
				if s.draft.Color != "never" {
					t.Fatal(s.draft.Color)
				}
			case "banner":
				if s.draft.Face.Preset != "kitty" || s.draft.Face.State != "happy" {
					t.Fatal(s.draft.Face)
				}
			case "server":
				if s.draft.Modules.Disk.Mount != "/" {
					t.Fatal(s.draft.Modules.Disk)
				}
			}
			if err := s.draft.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestFaceModesAndFormat(t *testing.T) {
	for _, mode := range []string{"preset", "seed", "random-once", "random-each-run"} {
		t.Run(mode, func(t *testing.T) {
			s := freshSession(t)
			s.derive()
			s.state = "face.mode"
			accept(t, s, mode)
			if mode == "seed" {
				accept(t, s, "a seed")
			}
			if mode == "preset" {
				accept(t, s, "kitty")
			}
			if !contains(s.selected, "face") || s.state != "face.sample" {
				t.Fatalf("%s %v", s.state, s.selected)
			}
			accept(t, s, "Accept")
			accept(t, s, "happy")
			before := s.sampleSeed
			if err := s.prepare(context.Background()); err != nil {
				t.Fatal(err)
			}
			if s.sampleSeed != before {
				t.Fatal("preview rerolled")
			}
			cfg, err := config.Parse("review", []byte(s.review))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "random-each-run" {
				if !cfg.Face.Random || cfg.Face.Seed != nil || cfg.Face.Preset != "" {
					t.Fatal(cfg.Face)
				}
			} else if cfg.Face.Random {
				t.Fatal(cfg.Face)
			}
			if mode == "random-once" && (*cfg.Face.Seed != before || len(before) != 32) {
				t.Fatal(cfg.Face)
			}
		})
	}
	s := freshSession(t)
	s.draft.Format = "{{hi}} {face}/{host} {face}"
	s.derive()
	s.setFace(false)
	if s.draft.Format != "{{hi}} /{host} " {
		t.Fatal(s.draft.Format)
	}
	s.setFace(true)
	if !strings.HasSuffix(s.draft.Format, "{face}") {
		t.Fatal(s.draft.Format)
	}
	s.state = "face.mode"
	s.term.runtime.Entropy = entropySource(strings.NewReader(""))
	if err := s.accept(context.Background(), "random-once"); err == nil || s.state != "face.error" {
		t.Fatalf("entropy %s %v", s.state, err)
	}
}
func TestFieldsOrderAndStyles(t *testing.T) {
	s := freshSession(t)
	s.draft.Format = "prefix {mem:percent}/{mem:used} {host}"
	s.derive()
	s.state = "modules.ready"
	accept(t, s, "mem,host")
	if s.state != "fields.ready" || s.queue[0] != "mem" {
		t.Fatal(s.state, s.queue)
	}
	accept(t, s, "percent")
	accept(t, s, "1 2")
	if s.draft.Format != "prefix {mem:percent}/{mem:used} {host}" {
		t.Fatal(s.draft.Format)
	}
	s.state = "modules.ready"
	accept(t, s, "host,mem,cpu")
	accept(t, s, "bar")
	accept(t, s, "threads")
	accept(t, s, "3 2 1")
	if s.draft.Format != "{cpu:threads} · {host} · {mem:bar}" {
		t.Fatal(s.draft.Format)
	}
	accept(t, s, "Continue")
	accept(t, s, "never")
	accept(t, s, "ascii")
	accept(t, s, "never")
	accept(t, s, "Edit styles")
	accept(t, s, "Edit foregrounds")
	accept(t, s, "terminal default")
	accept(t, s, "red")
	accept(t, s, "bright-blue")
	accept(t, s, "fish")
	if s.state != "review.ready" || s.draft.Style["cpu"].FG != "" || s.draft.Style["mem"].FG != "bright-blue" {
		t.Fatal(s.state, s.draft.Style)
	}
	if err := s.prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	accept(t, s, "Edit shell")
	accept(t, s, "none")
	accept(t, s, "Save")
	accept(t, s, "No")
	if s.state != "review.ready" {
		t.Fatal(s.state)
	}
	if _, err := permutation("1 1", []string{"a", "b"}); err == nil {
		t.Fatal("duplicates")
	}
	if _, err := permutation("1", []string{"a", "b"}); err == nil {
		t.Fatal("missing")
	}
	if _, err := permutation("3 1", []string{"a", "b"}); err == nil {
		t.Fatal("out of range")
	}
}
func TestBackNavigation(t *testing.T) {
	for _, tc := range []struct {
		from, to    string
		index, want int
	}{
		{"preset.ready", "destination.ready", 0, 0}, {"face.mode", "preset.ready", 0, 0}, {"face.identity", "face.mode", 0, 0}, {"face.sample", "face.identity", 0, 0}, {"face.expression", "face.sample", 0, 0}, {"modules.ready", "face.expression", 0, 0}, {"fields.ready", "fields.ready", 1, 0}, {"fields.ready", "modules.ready", 0, 0}, {"order.ready", "fields.ready", 0, 1}, {"settings.ready", "order.ready", 0, 0}, {"icons.ready", "settings.ready", 0, 0}, {"icons.set", "icons.ready", 0, 0}, {"color.ready", "icons.set", 0, 0}, {"styles.ready", "color.ready", 0, 0}, {"style.field", "style.field", 1, 0}, {"style.field", "styles.ready", 0, 0}, {"shell.ready", "styles.ready", 0, 0}, {"review.ready", "shell.ready", 0, 0}, {"confirm.ready", "review.ready", 0, 0},
	} {
		t.Run(tc.from+strconv.Itoa(tc.index), func(t *testing.T) {
			s := freshSession(t)
			s.state = tc.from
			s.queue = []string{"mem", "cpu"}
			s.index = tc.index
			s.faceMode = "seed"
			s.back()
			if s.state != tc.to || s.index != tc.want {
				t.Fatalf("%s %d", s.state, s.index)
			}
		})
	}
}
func TestPromptSnapshots(t *testing.T) {
	for _, width := range []int{19, 39, 80} {
		t.Run(string(rune('a'+width)), func(t *testing.T) {
			m := newPrompt(question{title: "Replace /home/ada/config.toml?", choices: []string{"No", "Yes"}, value: "No"}, width)
			snaptest.Snap(t, m.View().Content)
			snaptest.SnapCells(t, m.View().Content, snaptest.WithRoles(tuiweave.CatppuccinMocha()))
		})
	}
}
func TestPromptScenario(t *testing.T) {
	m := newPrompt(question{title: "Modules", choices: []string{"cpu", "mem"}, checked: []string{"mem"}, multiple: true}, 40)
	result := snaptest.RunScenario(m, snaptest.ScenarioStep{Name: "move", Msg: tea.KeyPressMsg{Code: tea.KeyDown}}, snaptest.ScenarioStep{Name: "toggle", Msg: tea.KeyPressMsg{Code: tea.KeySpace}}, snaptest.ScenarioStep{Name: "accept", Msg: tea.KeyPressMsg{Code: tea.KeyEnter}})
	snaptest.SnapScenario(t, result)
	last := requirePrompt(t, result.Model)
	if last.result.value != "" {
		t.Fatal(last.result)
	}
	m2, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil || !errors.Is(requirePrompt(t, m2).result.err, errBack) {
		t.Fatal("back")
	}
	if got := m.Inspect(); len(got.Children) != 1 {
		t.Fatal("hidden editor exposed")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestOutputFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	runtime := testRuntime(t, strings.Repeat("\n", 16)+"2\n", path)
	var diag bytes.Buffer
	code := Run(context.Background(), []string{"--config", path}, brokenWriter{}, &diag, runtime, testEnv())
	if code != 1 || !strings.Contains(diag.String(), "WAS saved") {
		t.Fatalf("%d %s", code, diag.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	runtime.Input = strings.NewReader("")
	if code = Run(context.Background(), []string{"--config", path}, io.Discard, brokenWriter{}, runtime, testEnv()); code != 1 {
		t.Fatal(code)
	}
}
func TestBasicValidation(t *testing.T) {
	runtime := testRuntime(t, "x\n2,2\n3\n2\n", t.TempDir())
	var out bytes.Buffer
	term := terminal{runtime: runtime, out: &out}
	defer func() { _ = term.close() }()
	got, err := term.basic(context.Background(), question{title: "Choose", choices: []string{"a", "b"}, value: "a"})
	if err != nil || got != "b" || !strings.Contains(out.String(), "without duplicates") {
		t.Fatalf("%q %v %s", got, err, out.String())
	}
	runtime.Input = strings.NewReader(":back\n:cancel\n")
	term = terminal{runtime: runtime, out: io.Discard}
	defer func() { _ = term.close() }()
	got, err = term.basic(context.Background(), question{title: "Seed", seed: true, value: "smol"})
	if err != nil || got != ":back" {
		t.Fatalf("literal seed %q %v", got, err)
	}
	runtime.Input = strings.NewReader("none\n")
	term = terminal{runtime: runtime, out: io.Discard}
	defer func() { _ = term.close() }()
	got, err = term.basic(context.Background(), question{title: "Modules", choices: []string{"cpu"}, checked: []string{"cpu"}, multiple: true})
	if err != nil || got != "" {
		t.Fatalf("none %q %v", got, err)
	}
}
func TestPromptEditing(t *testing.T) {
	m := newPrompt(question{title: "Seed", seed: true, value: "a", validate: func(v string) error {
		if len(v) < 2 {
			return errors.New("too short")
		}
		return nil
	}}, 0)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = requirePrompt(t, next)
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = requirePrompt(t, next)
	if cmd != nil || m.feedback != "too short" {
		t.Fatal(m.feedback)
	}
	next, _ = m.Update(tea.PasteMsg{Content: "\x1b"})
	m = requirePrompt(t, next)
	if m.editor.Value() != "a" {
		t.Fatal("control inserted")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = requirePrompt(t, next)
	next, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = requirePrompt(t, next)
	if m.result.value != "ab" || cmd == nil {
		t.Fatal(m.result)
	}
	if len(m.Inspect().Children) != 1 {
		t.Fatal("hidden menu exposed")
	}
	next, _ = newPrompt(question{title: "Cancel"}, 40).Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !errors.Is(requirePrompt(t, next).result.err, errCancel) {
		t.Fatal("cancel")
	}
}
func TestShellAndColor(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "none"} {
		snippet := shellInstructions(shell, "/home/a/b'c\\d $()")
		if shell == "none" {
			if snippet != "" {
				t.Fatal(snippet)
			}
		} else if !strings.Contains(snippet, "PATH") || !strings.Contains(snippet, "smol --config '") {
			t.Fatal(snippet)
		}
	}
	if detectShell("/bin/bash") != "bash" || detectShell("/bin/tcsh") != "none" {
		t.Fatal("detect")
	}
	runtime := testRuntime(t, "", t.TempDir())
	if previewColor("auto", runtime) || previewColor("never", runtime) || !previewColor("always", runtime) {
		t.Fatal("color policy")
	}
	runtime.Getenv = func(string) string { return "" }
	if !previewColor("auto", runtime) {
		t.Fatal("tty color")
	}
	if safe("hello\x1bworld") != "hello?world" {
		t.Fatal("sanitize")
	}
}
func TestAllQuestionsAndBackEdges(t *testing.T) {
	s := freshSession(t)
	s.derive()
	s.queue = []string{"mem"}
	for _, state := range []string{"destination.ready", "preset.ready", "face.mode", "face.identity", "face.sample", "face.expression", "modules.ready", "fields.ready", "order.ready", "color.ready", "styles.ready", "style.field", "shell.ready", "review.ready", "confirm.ready"} {
		s.state = state
		s.index = 0
		if q := s.question(); q.title == "" {
			t.Fatal(state)
		}
	}
	s.faceMode = "preset"
	s.state = "face.identity"
	if len(s.question().choices) == 0 {
		t.Fatal("presets")
	}
	s.faceMode = "random-each-run"
	s.state = "face.sample"
	if len(s.question().choices) != 2 {
		t.Fatal("reroll")
	}
	s.back()
	if s.state != "face.mode" {
		t.Fatal(s.state)
	}
	s.faceMode = "disabled"
	s.state = "modules.ready"
	s.back()
	if s.state != "face.mode" {
		t.Fatal(s.state)
	}
	s.queue = nil
	s.state = "order.ready"
	s.back()
	if s.state != "modules.ready" {
		t.Fatal(s.state)
	}
}

func requirePrompt(t *testing.T, model tea.Model) promptModel {
	t.Helper()
	prompt, ok := model.(promptModel)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	return prompt
}

func TestRecoverableSaveFailures(t *testing.T) {
	for _, kind := range []string{"write", "conflict"} {
		t.Run(kind, func(t *testing.T) {
			s := freshSession(t)
			s.derive()
			s.state = "confirm.ready"
			s.term.runtime.Input = strings.NewReader("2\n\n\n2\n")
			var out, diag bytes.Buffer
			s.term.out = &diag
			if err := s.prepare(context.Background()); err != nil {
				t.Fatal(err)
			}
			before, err := inspectTarget(s.path)
			if err != nil {
				t.Fatal(err)
			}
			s.snap = before
			fired := false
			s.store = store{fault: func(stage string) error {
				if fired {
					return nil
				}
				if kind == "write" && stage == "write" {
					fired = true
					return io.ErrShortWrite
				}
				if kind == "conflict" && stage == "recheck" {
					fired = true
					return os.WriteFile(s.path, []byte("external"), 0o600)
				}
				return nil
			}}
			if code := s.run(context.Background(), &out); code != 0 || out.String() != s.path+"\n" {
				t.Fatalf("%d %q %s", code, out.String(), diag.String())
			}
			if !fired || !strings.Contains(diag.String(), "not saved") {
				t.Fatal(diag.String())
			}
			if kind == "conflict" && !strings.Contains(diag.String(), "Destination changed") {
				t.Fatal(diag.String())
			}
			if err = s.term.close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestInspectionAndPreviewFailures(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	if err := os.WriteFile(path, []byte("not toml!"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, diag := runSetup(t, ":cancel\n", path)
	if code != 130 || out != "" || !strings.Contains(diag, "malformed") {
		t.Fatalf("%d %q %s", code, out, diag)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, path); err != nil {
		t.Fatal(err)
	}
	code, out, diag = runSetup(t, "2\n", path)
	if code != 1 || out != "" || !strings.Contains(diag, "Cannot inspect") {
		t.Fatalf("%d %q %s", code, out, diag)
	}
	s := freshSession(t)
	s.state = "review.ready"
	s.draft.Color = "invalid"
	s.term.runtime.Input = strings.NewReader("4\n")
	if code = s.run(context.Background(), io.Discard); code != 1 {
		t.Fatal(code)
	}
	_ = s.term.close()
}

type termination struct{}

func (termination) Error() string { return "SIGTERM" }
func (termination) ExitCode() int { return 143 }
func TestTerminationAndPendingWork(t *testing.T) {
	s := freshSession(t)
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(termination{})
	if code := s.run(ctx, io.Discard); code != 143 {
		t.Fatal(code)
	}
	s.term.runtime.Basic = false
	s.term.runtime.Getenv = func(string) string { return "" }
	called := false
	if err := s.term.work(context.Background(), "Test work", func() error { called = true; return nil }); err != nil || !called {
		t.Fatalf("%v called %v", err, called)
	}
}
func TestRichEOF(t *testing.T) {
	runtime := testRuntime(t, "", t.TempDir())
	runtime.Basic = false
	runtime.Getenv = func(string) string { return "xterm-256color" }
	var out bytes.Buffer
	term := terminal{runtime: runtime, out: &out}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := term.ask(ctx, question{title: "EOF", choices: []string{"No", "Yes"}, value: "No"})
	if !errors.Is(err, errCancel) {
		t.Fatalf("EOF: %v", err)
	}
}

func TestWizardPromptStates(t *testing.T) {
	for _, width := range []int{39, 80} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			for _, state := range []string{"destination.ready", "preset.ready", "face.mode", "face.identity", "face.sample", "face.expression", "modules.ready", "fields.ready", "order.ready", "color.ready", "styles.ready", "style.field", "shell.ready", "review.ready", "confirm.ready"} {
				t.Run(state, func(t *testing.T) {
					s := freshSession(t)
					s.path = "/home/ada/.config/smol/config.toml"
					s.derive()
					s.state = state
					s.queue = []string{"mem"}
					s.faceMode = "seed"
					model := newPrompt(s.question(), width)
					snaptest.Snap(t, model.View().Content)
				})
			}
		})
	}
}
func TestWorkPrompt(t *testing.T) {
	model := workModel{spinner: spinner.New(tuiweave.CatppuccinMocha()), label: "Saving configuration"}
	snaptest.Snap(t, model.View().Content)
	snaptest.SnapCells(t, model.View().Content, snaptest.WithRoles(tuiweave.CatppuccinMocha()))
	next, cmd := model.Update(workDone{})
	if cmd == nil || next.View().Content != "" {
		t.Fatal("work did not finish")
	}
}

func TestCommitSignalOutcome(t *testing.T) {
	for _, stage := range []string{"write", "published"} {
		t.Run(stage, func(t *testing.T) {
			s := freshSession(t)
			s.derive()
			s.state = "confirm.ready"
			s.term.runtime.Input = strings.NewReader("2\n")
			if err := s.prepare(context.Background()); err != nil {
				t.Fatal(err)
			}
			snap, err := inspectTarget(s.path)
			if err != nil {
				t.Fatal(err)
			}
			s.snap = snap
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			s.store = store{fault: func(name string) error {
				if name == stage {
					cancel(termination{})
				}
				return nil
			}}
			var out bytes.Buffer
			code := s.run(ctx, &out)
			if stage == "published" {
				if code != 0 || out.String() != s.path+"\n" {
					t.Fatalf("publication lost: %d %q", code, out.String())
				}
			} else if code != 143 || out.Len() != 0 {
				t.Fatalf("precommit signal %d %q", code, out.String())
			}
			_ = s.term.close()
		})
	}
}
func TestRefreshFailureDoesNotCommit(t *testing.T) {
	s := freshSession(t)
	s.derive()
	s.state = "save.pending"
	s.term.runtime.Input = strings.NewReader("\n2\n")
	if err := os.Symlink(filepath.Dir(s.path), s.path); err != nil {
		t.Fatal(err)
	}
	if err := s.refresh(context.Background()); err == nil {
		t.Fatal("unsafe refresh accepted")
	}
	info, err := os.Lstat(s.path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("refresh changed unsafe target: %v", err)
	}
	_ = s.term.close()
}
func TestRerollIsExplicit(t *testing.T) {
	s := freshSession(t)
	s.derive()
	s.state = "face.mode"
	s.term.runtime.Entropy = entropySource(bytes.NewReader(append(bytes.Repeat([]byte{1}, 16), bytes.Repeat([]byte{2}, 16)...)))
	accept(t, s, "random-each-run")
	first := s.sampleSeed
	accept(t, s, "Reroll")
	if first == s.sampleSeed || s.draft.Face.Seed != nil || !s.draft.Face.Random {
		t.Fatal("reroll did not replace only sample identity")
	}
}

func TestQualifiedActions(t *testing.T) {
	model := newPrompt(question{title: "Confirm", choices: []string{"No", "Yes"}, value: "No"}, 80)
	next, cmd := model.Update(inspect.Invoke("editor.clear"))
	if cmd != nil || requirePrompt(t, next).menu.SelectedID() != "No" {
		t.Fatal("hidden action routed")
	}
	next, cmd = model.Update(inspect.Invoke("choices.next"))
	model = requirePrompt(t, next)
	if cmd != nil || model.menu.SelectedID() != "Yes" {
		t.Fatal("visible navigation failed")
	}
	next, cmd = model.Update(inspect.Invoke("choices.activate"))
	model = requirePrompt(t, next)
	if cmd == nil || model.result.value != "Yes" {
		t.Fatal("activation did not submit")
	}
	// Explicitly deliver the command's outcome; the scenario harness does not.
	next, _ = model.Update(cmd())
	if !requirePrompt(t, next).finished {
		t.Fatal("lost completion")
	}
}

func entropySource(reader io.Reader) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return face.RandomSeed(reader)
	}
}
func TestCancellationDuringEntropy(t *testing.T) {
	s := freshSession(t)
	s.faceMode = "random-once"
	s.term.runtime.Entropy = func(ctx context.Context) (string, error) { <-ctx.Done(); return "", ctx.Err() }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := s.roll(ctx); !errors.Is(err, context.DeadlineExceeded) || s.sampleSeed != "" {
		t.Fatalf("%q %v", s.sampleSeed, err)
	}
	if _, err := os.Stat(s.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestResizeRetainsInput(t *testing.T) {
	model := newPrompt(question{title: "Seed", seed: true, value: "kept seed"}, 80)
	next, cmd := model.Update(tea.WindowSizeMsg{Width: 19, Height: 10})
	got := requirePrompt(t, next)
	if cmd == nil || !errors.Is(got.result.err, errBasic) || got.editor.Value() != "kept seed" {
		t.Fatal("resize lost input")
	}
}

func TestWelcome(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		width, rows                      int
		basic, noColor, colored, compact bool
	}{
		{name: "gradient", width: 80, rows: 24, colored: true},
		{name: "no-color", width: 80, rows: 24, noColor: true},
		{name: "basic", width: 80, rows: 24, basic: true},
		{name: "narrow", width: 19, rows: 24, compact: true},
		{name: "short", width: 80, rows: 10, compact: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			term := terminal{out: &out, runtime: Runtime{Width: tc.width, Rows: tc.rows, Basic: tc.basic, Getenv: func(k string) string {
				if k == "NO_COLOR" && tc.noColor {
					return "1"
				}
				return ""
			}}}
			if err := term.welcome(); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if strings.Contains(text, "\x1b[") != tc.colored {
				t.Fatalf("unexpected banner color: %q", text)
			}
			if strings.HasPrefix(text, "\nsmol\n") != tc.compact {
				t.Fatalf("unexpected banner size: %q", text)
			}
			for line := range strings.SplitSeq(text, "\n") {
				if ansi.StringWidth(line) > tc.width {
					t.Fatalf("banner exceeds width %d: %q", tc.width, line)
				}
			}
			snaptest.Snap(t, text)
		})
	}
}

func TestSpacedPromptGeometry(t *testing.T) {
	for _, choices := range [][]string{nil, {"No", "Yes"}} {
		m := newPrompt(question{title: "A long question that wraps onto several lines", choices: choices, value: "No"}, 19)
		node := m.Inspect().Children[0]
		lines := strings.Split(ansi.Strip(m.View().Content), "\n")
		if node.Bounds.Y < 2 || node.Bounds.Y+node.Bounds.Height > len(lines) {
			t.Fatalf("invalid component bounds: %+v", node.Bounds)
		}
		if strings.TrimSpace(lines[node.Bounds.Y-1]) != "" || !strings.Contains(lines[node.Bounds.Y], "◆") || !strings.Contains(lines[node.Bounds.Y+1], "No") {
			t.Fatalf("component bounds do not match spaced view: %+v %q", node.Bounds, lines)
		}
		m.height = len(lines)
		if !m.needsBasic() {
			t.Fatal("short terminal must use numbered input")
		}
	}
}

func TestInlineReorder(t *testing.T) {
	s := freshSession(t)
	s.derive()
	s.state = "order.ready"
	q := s.question()
	m := newPrompt(q, 80)
	if got := m.Inspect().Children[0]; got.Kind != "reorder" || !got.Focused {
		t.Fatalf("order field: %+v", got)
	}
	// A semantic move and keyboard grab both change the same submitted order.
	next, cmd := m.Update(inspect.Invoke("order.move-down"))
	m = requirePrompt(t, next)
	if cmd == nil || m.currentValue() == q.value {
		t.Fatal("semantic move did not reorder")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = requirePrompt(t, next)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = requirePrompt(t, next)
	moved := m.currentValue()
	if moved == q.value {
		t.Fatal("grabbed item did not move")
	}
	if _, err := permutation(moved, s.selected); err != nil {
		t.Fatal(err)
	}
	// Resizing hands the current order to basic input without resetting it.
	next, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 12})
	m = requirePrompt(t, next)
	if !errors.Is(m.result.err, errBasic) || m.currentValue() != moved {
		t.Fatal("resize lost order")
	}
	q.value = moved
	restored := newPrompt(q, 80)
	if restored.currentValue() != moved {
		t.Fatal("recreated field lost order")
	}
	next, _ = restored.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if requirePrompt(t, next).result.value != moved {
		t.Fatal("submitted stale order")
	}
	var out bytes.Buffer
	runtime := testRuntime(t, "\n", filepath.Join(t.TempDir(), "config.toml"))
	term := terminal{runtime: runtime, out: &out}
	defer func() {
		if err := term.close(); err != nil {
			t.Error(err)
		}
	}()
	got, err := term.basic(context.Background(), q)
	if err != nil || got != moved || !strings.Contains(out.String(), "1. ") {
		t.Fatalf("basic order = %q, %v; %s", got, err, out.String())
	}
}

func TestInlineMultiselectActions(t *testing.T) {
	m := newPrompt(question{title: "Modules", choices: []string{"cpu", "mem"}, multiple: true}, 80)
	next, cmd := m.Update(inspect.Invoke("modules.toggle"))
	m = requirePrompt(t, next)
	if cmd == nil || m.currentValue() != "cpu" {
		t.Fatal("semantic checkbox did not toggle")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = requirePrompt(t, next)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = requirePrompt(t, next)
	next, _ = m.Update(inspect.Invoke("modules.submit"))
	if requirePrompt(t, next).result.value != "cpu,mem" {
		t.Fatal("submitted stale selection")
	}
}
