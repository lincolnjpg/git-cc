package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/skalt/git-cc/pkg/config"
	"github.com/skalt/git-cc/pkg/tui_breaking_change_input"
	"github.com/skalt/git-cc/pkg/tui_description_editor"
	"github.com/skalt/git-cc/pkg/tui_single_select"
)

type componentIndex int

const ( // the order of the components
	commitTypeIndex componentIndex = iota
	scopeIndex
	shortDescriptionIndex
	breakingChangeIndex
	// body omitted -- performed by GIT_EDITOR
	doneIndex
)

type InputComponent interface {
	View() string
	// Update(tea.Msg) (tea.Model, tea.Cmd)

	Value() string
	// // tea.Model       // Init() tea.Cmd, Update(tea.Msg) (tea.Model, tea.Cmd), View() string
	// Focus() tea.Cmd // should focus any internals, i.e. text inputs
	// // Cancel()  // should clean up any resources (i.e. open channels)
	// Submit()  // send the input to the output channel
}

type model struct {
	// components [done]InputComponent
	commit  [doneIndex]string
	viewing componentIndex

	typeInput           tui_single_select.Model
	scopeInput          tui_single_select.Model
	descriptionInput    tui_description_editor.Model
	breakingChangeInput tui_breaking_change_input.Model

	choice chan string
}

func (m model) ready() bool {
	return len(m.commit[commitTypeIndex]) > 0 && len(m.commit[shortDescriptionIndex]) > 0
}

func (m model) contextValue() string {
	result := strings.Builder{}
	result.WriteString(m.commit[commitTypeIndex])
	scope := m.commit[scopeIndex]
	if scope != "" {
		result.WriteString(fmt.Sprintf("(%s)", scope))
	}
	return result.String()
}
func (m model) value() string {
	result := strings.Builder{}
	result.WriteString(m.contextValue())
	breakingChange := m.commit[breakingChangeIndex]
	if breakingChange != "" {
		result.WriteRune('!')
	}
	result.WriteString(fmt.Sprintf(": %s\n", m.commit[shortDescriptionIndex]))
	if breakingChange != "" {
		result.WriteString(fmt.Sprintf("\nBREAKING CHANGE: %s\n", breakingChange))
		// TODO: handle muliple breaking change footers(?)
	}
	return result.String()
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) currentComponent() InputComponent {
	return [...]InputComponent{
		m.typeInput,
		m.scopeInput,
		m.descriptionInput,
		m.breakingChangeInput,
	}[m.viewing]
}

func main() {
	choice := make(chan string, 1)
	m := initialModel(choice)
	ui := tea.NewProgram(m)
	if err := ui.Start(); err != nil {
		log.Fatal(err)
	}
	if r := <-choice; r == "" {
		close(choice)
		os.Exit(1) // no submission
	} else {
		fmt.Printf("\n---\nYou chose `%s`\n", r)
	}
}

// Pass a channel to the model to listen to the result value. This is a
// function that returns the initialize function and is typically how you would
// pass arguments to a tea.Init function.
func initialModel(choice chan string) model {
	cfg := config.Init()
	data := config.Lookup(cfg)
	typeModel := tui_single_select.NewModel(
		termenv.String("select a commit type: ").Faint().String(),
		data.CommitTypes)
	scopeModel := tui_single_select.NewModel(
		termenv.String("select a scope:").Faint().String(),
		data.Scopes) // TODO: skip scopes none present?
	descModel := tui_description_editor.NewModel(data.HeaderMaxLength, data.EnforceMaxLength)
	bcModel := tui_breaking_change_input.NewModel()
	return model{
		choice:              choice,
		commit:              [doneIndex]string{}, // TODO: read initial state from cli
		typeInput:           typeModel,
		scopeInput:          scopeModel,
		descriptionInput:    descModel,
		breakingChangeInput: bcModel,
		viewing:             commitTypeIndex}
}

func (m model) updateCurrentInput(msg tea.Msg) model {
	switch m.viewing {
	case commitTypeIndex:
		m.typeInput, _ = m.typeInput.Update(msg)
	case scopeIndex:
		m.scopeInput, _ = m.scopeInput.Update(msg)
	case shortDescriptionIndex:
		m.descriptionInput, _ = m.descriptionInput.Update(msg)
	case breakingChangeIndex:
		m.breakingChangeInput, _ = m.breakingChangeInput.Update(msg)
	}
	return m
}
func (m model) done() (model, tea.Cmd) {
	if m.ready() {
		m.choice <- m.value()
	} else {
		m.choice <- ""
	}
	return m, tea.Quit
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			return m.done()
		case tea.KeyShiftTab:
			if m.viewing > commitTypeIndex {
				m.viewing--
			}
			return m, cmd
		case tea.KeyEnter:
			switch m.viewing {
			default:
				m.commit[m.viewing] = m.currentComponent().Value()
				m.viewing++
			case scopeIndex:
				m.descriptionInput = m.descriptionInput.SetPrefix(
					m.contextValue() + ": ",
				)
				m.viewing++
				return m, cmd
			case breakingChangeIndex:
				m.commit[breakingChangeIndex] = m.breakingChangeInput.Value()
				if m.ready() {
					return m.done()
				} else {
					err := fmt.Errorf("required")
					if m.commit[commitTypeIndex] == "" {
						m.viewing = commitTypeIndex
						m.typeInput = m.typeInput.SetErr(err)
					} else if m.commit[shortDescriptionIndex] == "" {
						m.viewing = shortDescriptionIndex
						m.descriptionInput = m.descriptionInput.SetErr(err)
					}
					return m, cmd
				}
			case doneIndex:
				fmt.Printf("%d > done", m.viewing)
				os.Exit(1)
			}
			return m, cmd
		default:
			m = m.updateCurrentInput(msg)
		}
	}
	return m, cmd
}

func (m model) View() string {
	return m.currentComponent().View()
}