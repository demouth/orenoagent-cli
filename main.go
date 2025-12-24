package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/demouth/orenoagent-go"
	"github.com/openai/openai-go/v3"
	"github.com/tectiv3/websearch"
	"github.com/tectiv3/websearch/provider"
)

const gap = "\n\n"

type functionCallMessage struct {
	message string
}
type answerMessage struct {
	message string
}
type reasoningMessage struct {
	message string
}

var program *tea.Program
var agent *orenoagent.Agent
var ctx context.Context
var tools = []orenoagent.Tool{
	{
		Name:        "currentTime",
		Description: "Get the current date and time with timezone in a human-readable format.",
		Function: func(_ string) string {
			return time.Now().Format(time.RFC3339)
		},
	},
	{
		// NOTE: This is a sample function. Do not use it in production environments.

		Name:        "webSearch",
		Description: "Get the current date and time with timezone in a human-readable format.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]string{
					"type":        "string",
					"description": "web search keyword.",
				},
			},
			"required": []string{"keyword"},
		},
		Function: func(args string) string {
			var param struct {
				Keyword string
			}
			err := json.Unmarshal([]byte(args), &param)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}

			type result struct {
				Title   string
				Link    string
				Snippet string
			}
			results := []result{}
			web := websearch.New(provider.NewUnofficialDuckDuckGo())
			res, err := web.Search(param.Keyword, 10)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}
			for _, ddgor := range res {
				r := result{
					Title:   ddgor.Title,
					Link:    ddgor.Link.String(),
					Snippet: ddgor.Description,
				}
				results = append(results, r)
			}
			v, _ := json.Marshal(results)

			return string(v)
		},
	},
	{
		Name:        "WebReader",
		Description: "Reads and returns the content from the specified URL",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]string{
					"type":        "string",
					"description": "URL of the page to retrieve",
				},
			},
			"required": []string{"url"},
		},
		Function: func(args string) string {
			var param struct {
				Url string
			}
			err := json.Unmarshal([]byte(args), &param)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}

			req, _ := http.NewRequest("GET", param.Url, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}
			defer resp.Body.Close()
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}

			return string(bodyBytes)
		},
	},
}

func main() {
	model := initialModel()
	program = tea.NewProgram(model)

	client := openai.NewClient()
	ctx = context.Background()
	agent = orenoagent.NewAgent(client, tools, true)

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
				p.Send(answerMessage{message: r.String() + "\n"})
			case *orenoagent.ReasoningResult:
				p.Send(reasoningMessage{message: r.String() + "\n"})
			case *orenoagent.FunctionCallResult:
				p.Send(functionCallMessage{message: r.String() + "\n"})
			}
		}
	}()
}

type (
	errMsg error
)

type model struct {
	viewport    viewport.Model
	messages    []string
	textarea    textarea.Model
	senderStyle lipgloss.Style
	agentStyle  lipgloss.Style
	err         error
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
		textarea:    ta,
		messages:    []string{},
		viewport:    vp,
		senderStyle: lipgloss.NewStyle().Background(lipgloss.Color("5")),
		agentStyle:  lipgloss.NewStyle().Background(lipgloss.Color("2")),
		err:         nil,
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
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
		}
		m.viewport.GotoBottom()
	case answerMessage:
		m.messages = append(m.messages, m.agentStyle.Render("Agent"), msg.message)
		m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
		m.viewport.GotoBottom()
		return m, nil
	case reasoningMessage:
		m.messages = append(m.messages, m.agentStyle.Render("Reasoning"), msg.message)
		m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
		m.viewport.GotoBottom()
		return m, nil
	case functionCallMessage:
		m.messages = append(m.messages, m.agentStyle.Render("Function Call"), msg.message)
		m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
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
			message := m.textarea.Value() + "\n"
			m.messages = append(m.messages,
				m.senderStyle.Render("You:"),
				message)
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
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

func (m model) View() string {
	return fmt.Sprintf(
		"%s%s%s",
		m.viewport.View(),
		gap,
		m.textarea.View(),
	)
}
