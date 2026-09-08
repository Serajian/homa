package ui

import (
	"context"
	"errors"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/Serajian/homa/internal/config"
)

// settingsForm is the two questions, with the current values as defaults
// so changing one means pressing Enter past the other. The first run and
// the settings screen ask the same two; the wording is version 1's.
func settingsForm(title string, current *config.Config) *form {
	return newForm(
		title,
		field{
			label: "The name shown beside your messages",
			def:   current.Nick,
			check: func(s string) error {
				candidate := *current
				candidate.Nick = s
				return candidate.Validate()
			},
		},
		field{label: "Where received files should go", def: current.DownloadDir},
	)
}

// applySettings writes a finished settings form into a copy of the
// settings and saves it. The copy is returned rather than the original
// written through, as version 1 did, so nothing else holding the pointer
// sees a half-changed value.
func applySettings(current *config.Config, answers []string) (*config.Config, error) {
	edited := *current
	edited.Nick, edited.DownloadDir = answers[0], answers[1]
	if err := edited.Save(); err != nil {
		return nil, err
	}
	return &edited, nil
}

// setupModel is the first run: the banner, a line saying there are two
// questions, and the two questions. A program of its own, because it runs
// before the identity exists and before there is anything to listen on.
type setupModel struct {
	st            *styles
	form          *form
	width, height int
	cfg           *config.Config
	err           error
	canceled      bool
}

func (m setupModel) Init() tea.Cmd { return nil }

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == keyQuit {
			m.canceled = true
			return m, tea.Quit
		}
		done, cancel := m.form.update(m.st, msg)
		if cancel {
			m.canceled = true
			return m, tea.Quit
		}
		if done {
			m.cfg, m.err = applySettings(config.Default(), m.form.answers())
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m setupModel) View() tea.View {
	body := "\n" + bannerBlock(m.st, m.width) + "\n" +
		markInfo + m.st.dim.Render("This is the first run, so two questions.") + "\n" +
		m.form.view(m.st)
	v := tea.NewView(frame(m.width, m.height,
		header(m.st, m.width, brand(m.st), m.st.dim.Render("first run")), body,
		footer(m.st, m.width, m.st.keys("Enter", "next", "Esc", "stop"))))
	v.AltScreen = false
	return v
}

// ErrSetupCanceled is the first run left before its questions were answered.
var ErrSetupCanceled = errors.New("setup was not finished")

// RunSetup asks the first-run questions on the screen and saves the
// answers. It is its own program, run by cmd/homa before anything else
// exists; the main program starts afterwards with the settings it saved.
func RunSetup(ctx context.Context, noColor bool) (*config.Config, error) {
	st := newStyles(unicodeLocale(os.Getenv))
	opts := []tea.ProgramOption{tea.WithContext(ctx)}
	if noColor {
		opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
	}

	_, _ = os.Stdout.WriteString(clearScreen) // see clearScreen

	m := setupModel{st: st, form: settingsForm("Welcome", config.Default())}
	final, err := tea.NewProgram(m, opts...).Run()
	if err != nil {
		return nil, ErrSetupCanceled
	}
	out, ok := final.(setupModel)
	if !ok || out.canceled {
		return nil, ErrSetupCanceled
	}
	if out.err != nil {
		return nil, out.err
	}
	return out.cfg, nil
}
