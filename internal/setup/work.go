package setup

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/ishan5ain/tuiweave"
	"github.com/ishan5ain/tuiweave/spinner"
)

type workDone struct{ err error }
type workModel struct {
	spinner spinner.Model
	label   string
	done    bool
	result  <-chan error
}

func (m workModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick(), func() tea.Msg { return workDone{err: <-m.result} })
}
func (m workModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(workDone); ok {
		m.done = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}
func (m workModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(m.spinner.View() + " " + m.label)
}

// work owns the transient line, awaits its worker even on cancellation, and
// never lets the terminal renderer race an effect's output.
func (t *terminal) work(ctx context.Context, label string, fn func() error) error {
	if t.bridge != nil {
		select {
		case t.bridge.requests <- wizardRequest{status: label + "…"}:
		case <-ctx.Done():
			return ctx.Err()
		}
		return fn()
	}
	if t.runtime.Basic || t.runtime.Width < 40 {
		if err := t.say(label + "..."); err != nil {
			return err
		}
		return fn()
	}
	result := make(chan error, 1)
	finished := make(chan struct{})
	var workErr error
	go func() { defer close(finished); workErr = fn(); result <- workErr }()
	model := workModel{spinner: spinner.New(tuiweave.CatppuccinMocha()), label: label, result: result}
	opts := []tea.ProgramOption{tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(t.out), tea.WithoutSignalHandler(), tea.WithoutCatchPanics()}
	if t.runtime.Getenv("NO_COLOR") != "" {
		opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
	}
	_, terminalErr := tea.NewProgram(model, opts...).Run()
	<-finished
	return errors.Join(workErr, terminalErr)
}
