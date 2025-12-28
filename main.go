package main

import (
	"context"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/demouth/orenoagent-go"
	"github.com/openai/openai-go/v3"
)

const gap = "\n\n"

type functionCallMessage struct {
	message string
}

func (m functionCallMessage) Content() string {
	s := lipgloss.NewStyle().Background(lipgloss.Color("2")).Render(" Function Call ") + " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.message)
	return s
}

type answerMessage struct {
	message string
}

func (m answerMessage) Content() string {
	return lipgloss.NewStyle().Background(lipgloss.Color("2")).Render(" Agent ") + " " + m.message
}

type reasoningMessage struct {
	message string
}

func (m reasoningMessage) Content() string {
	s := lipgloss.NewStyle().Background(lipgloss.Color("2")).Render(" Reasoning ") +
		" " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.message)
	return s
}

var program *tea.Program
var agent *orenoagent.Agent
var ctx context.Context

func main() {
	model := initialModel()
	program = tea.NewProgram(model)

	client := openai.NewClient()
	ctx = context.Background()
	agent = orenoagent.NewAgent(client, Tools, true)

	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}

func ask(question string, p *tea.Program) {

	go func() {
		results, _ := agent.Ask(ctx, question)
		for result := range results {
			switch r := result.(type) {
			case *orenoagent.MessageResult:
				p.Send(answerMessage{message: r.String()})
			case *orenoagent.ReasoningResult:
				p.Send(reasoningMessage{message: r.String()})
			case *orenoagent.FunctionCallResult:
				p.Send(functionCallMessage{message: r.String()})
			}
		}
	}()
}

type (
	errMsg error
)

type model struct {
	viewport viewport.Model
	messages []string
	textarea textarea.Model
	err      error
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 280

	ta.SetWidth(30)
	ta.SetHeight(3)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	ta.ShowLineNumbers = false

	vp := viewport.New(30, 5)
	vp.SetContent(`Welcome to ore-no-agent!
Type a message and press Enter to send.`)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		textarea: ta,
		messages: []string{},
		viewport: vp,
		err:      nil,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap)

		if len(m.messages) > 0 {
			// Wrap content before setting it.
			m.render()
		}
		m.viewport.GotoBottom()
	case answerMessage:
		m.messages = append(m.messages, msg.Content())
		m.render()
		m.viewport.GotoBottom()
		return m, nil
	case reasoningMessage:
		m.messages = append(m.messages, msg.Content())
		m.render()
		m.viewport.GotoBottom()
		return m, nil
	case functionCallMessage:
		m.messages = append(m.messages, msg.Content())
		m.render()
		m.viewport.GotoBottom()
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlJ:
			m.textarea.InsertString("\n")
		case tea.KeyCtrlC, tea.KeyEsc:
			fmt.Println(m.textarea.Value())
			return m, tea.Quit
		case tea.KeyEnter:
			message := m.textarea.Value()
			m.messages = append(m.messages, lipgloss.NewStyle().Background(lipgloss.Color("5")).Render(" You ")+" "+message)
			m.render()
			m.textarea.Reset()
			ask(message, program)
			m.viewport.GotoBottom()
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *model) render() {
	var s string
	for _, message := range m.messages {
		s = s + lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("241")).
			PaddingTop(1).
			PaddingLeft(1).
			PaddingRight(1).
			Render(
				lipgloss.NewStyle().Width(m.viewport.Width).Render(message),
			)
	}
	m.viewport.SetContent(s)
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s%s%s",
		m.viewport.View(),
		gap,
		m.textarea.View(),
	)
}
