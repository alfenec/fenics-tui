package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages for async operations
type copyDoneMsg struct {
	err error
}
type notebookDoneMsg struct {
	err error
}
type resultsDoneMsg struct {
	err    error
	result Result
}

// =========================
// JSON RESULT FROM NOTEBOOK
// =========================

type Result struct {
	Comparaison struct {
		AnalytiqueMM   float64 `json:"analytique_mm"`
		FemMM          float64 `json:"fem_mm"`
		ErreurPourcent float64 `json:"erreur_pourcent"`
	} `json:"comparaison"`

	Contrainte struct {
		LimiteMPa           float64 `json:"limite_MPa"`
		SigmaMaxMPa         float64 `json:"sigma_max_MPa"`
		CoefficientSecurite float64 `json:"coefficient_securite"`
		OK                  bool    `json:"ok"`
	} `json:"contrainte"`

	Fleche struct {
		DeltaMaxMM float64 `json:"delta_max_mm"`
		DeltaLimMM float64 `json:"delta_lim_mm"`
		OK         bool    `json:"ok"`
	} `json:"fleche"`

	StatusGlobal bool `json:"status_global"`
}

type beamType struct {
	id       string
	title    string
	schema   string
	appuiA   string
	appuiB   string
	loadType string
}

var beams = []beamType{
	{
		id:       "console_point",
		title:    "Console",
		schema:   "x=0           x=a       x=L\n|───────────────↓────────\n               F",
		appuiA:   "fixed",
		appuiB:   "free",
		loadType: "point_load",
	},
	{
		id:       "bi_encastre_point",
		title:    "Bi-encastré",
		schema:   "x=0           x=a       x=L\n|───────────────↓───────|\n               F",
		appuiA:   "fixed",
		appuiB:   "fixed",
		loadType: "point_load",
	},
	{
		id:       "encastre_rotule_point",
		title:    "Encastré-rotulé",
		schema:   "x=0           x=a       x=L\n|───────────────↓───────●\n               F",
		appuiA:   "fixed",
		appuiB:   "pin",
		loadType: "point_load",
	},
	{
		id:       "console_distribued",
		title:    "Console",
		schema:   "x=0     x=a      x=b    x=L\n|───────┬────────┬───────\n         ↓↓↓↓↓↓↓↓\n            q",
		appuiA:   "fixed",
		appuiB:   "free",
		loadType: "distributed_load",
	},
	{
		id:       "bi_encastre_distributed",
		title:    "Bi-encastré",
		schema:   "x=0     x=a      x=b    x=L\n|───────┬────────┬──────|\n         ↓↓↓↓↓↓↓↓\n            q",
		appuiA:   "fixed",
		appuiB:   "fixed",
		loadType: "distributed_load",
	},
	{
		id:       "encastre_rotule_distributed",
		title:    "Encastré-rotulé",
		schema:   "x=0     x=a      x=b    x=L\n|───────┬────────┬──────●\n         ↓↓↓↓↓↓↓↓\n            q",
		appuiA:   "fixed",
		appuiB:   "pin",
		loadType: "distributed_load",
	},
}

type sectionType struct {
	id     string
	title  string
	schema string
}

var sectionCards = []sectionType{
	{
		id:    "rectangular",
		title: "Rectangulaire",
		schema: `
		
   ███████████████ ↑
   ███████████████ │
   ███████████████ │
   ███████████████ │
   ███████████████ h
   ███████████████ │
   ███████████████ │
   ███████████████ │
   ███████████████ ↓
   ←───── b ─────→`,
	},
	{
		id:    "circular",
		title: "Circulaire",
		schema: `

      ▄▄██████▄▄ ↙2r
    ▄████████████▄
   ████████████████
  ██████████████████
  ██████████████████
  ██████████████████
   ████████████████
    ▀████████████▀
    ↗ ▀▀██████▀▀
`,
	},
	{
		id:    "rectangular_hollow",
		title: "Rectangular creux",
		schema: `

   ███████████████ ↑
   ██           ██ │
   ██ t         ██ │
  →██←          ██ │
   ██           ██ h
   ██           ██ │
   ██           ██ │
   ██           ██ │
   ███████████████ ↓
   ←───── b ─────→`,
	},
	{
		id:    "circular_hollow",
		title: "Circulaire creux",
		schema: `

      ▄▄█▀▀▀▀█▄▄ ↙re
    ▄█▀        ▀█▄
   █▀            ▀█
  █▀              ▀█
  █        ┼──ri──→█
  █▄              ▄█
   █▄            ▄█
    ▀█▄        ▄█▀
    ↗ ▀▀█▄▄▄▄█▀▀
`,
	},
	{
		id:    "i_beam",
		title: "Poutre en I",
		schema: `
    ↓
   ███████████████ ↑
    ↑     █        │
   tf     █        │
          █ tw     │
         →█←       h
          █        │
          █        │
          █        │
   ███████████████ ↓
   ←───── b ─────→`,
	},
	{
		id:    "u_beam",
		title: "Poutre en U",
		schema: `
      ↓
   ███████████████ ↑
   █  ↑            │
   █  tf           │
   █               │
   █ tw            h
  →█←              │
   █               │
   █               │
   ███████████████ ↓
   ←───── b ─────→`,
	},
}

var sectionTypes = []string{
	"Rectangulaire plein",
	"Circulaire plein",
	"Circulaire creux",
	"Rectangulaire creux",
	"Poutre en I",
	"Poutre en U",
}

var (
	selectedBeam           int
	selectedSection        int
	length, b, h, r, r_int string
	tf, tw                 string
	E, nu                  string
	P, xP                  string
	q, xEnd                string
)

type step int

const (
	stepSelectBeam step = iota
	stepLength
	stepSection
	stepGeometry
	stepMaterial
	stepNu
	stepLoadP
	stepLoadX
	stepLoadQ
	stepLoadXEnd
	stepSummary
	stepConfirm
	stepCopyToK8s
	stepRunNotebook
	stepGetResults
	stepDone
)

type model struct {
	step          step
	beamIndex     int
	sectionIndex  int
	inputField    string
	inputValue    string
	spinner       spinner.Model
	notebookCells int
	notebookDone  int
	statusMsg     string
	result        Result
}

func newModel() *model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return &model{
		step:          stepSelectBeam,
		beamIndex:     0,
		spinner:       s,
		notebookCells: 31,
		notebookDone:  0,
		statusMsg:     "Prêt",
	}
}

func (m *model) Init() tea.Cmd {
	return spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	case copyDoneMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}

		m.step = stepRunNotebook
		m.statusMsg = "Notebook en cours..."
		return m, m.runNotebook()

	case notebookDoneMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}

		m.step = stepGetResults
		m.statusMsg = "Récupération résultats..."
		return m, m.getResults()

	case resultsDoneMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}

		fmt.Printf("DEBUG resultsDoneMsg: result=%+v\n", msg.result)
		m.result = msg.result
		m.step = stepDone

	case spinner.TickMsg:
		sm, cmd := m.spinner.Update(msg)
		m.spinner = sm
		return m, cmd

	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "tab":
			return m.handleTab()
		case "right", "l":
			return m.handleRight()
		case "left", "h":
			return m.handleLeft()
		case "down", "j":
			return m.handleDown()
		case "up", "k":
			return m.handleUp()
		case "backspace":
			if len(m.inputValue) > 0 {
				m.inputValue = m.inputValue[:len(m.inputValue)-1]
			}
		}
		if m.step == stepLength || m.step == stepSection || m.step == stepGeometry || m.step == stepMaterial || m.step == stepNu || m.step == stepLoadP || m.step == stepLoadX || m.step == stepLoadQ || m.step == stepLoadXEnd {
			m.inputValue += key
		}
	}

	return m, nil

}

func (m *model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSelectBeam:
		selectedBeam = m.beamIndex
		m.inputValue = ""
		m.step = stepLength
	case stepLength:
		if m.inputValue == "" {
			return m, nil
		}
		length = m.inputValue
		m.inputValue = ""
		m.step = stepSection
	case stepSection:
		selectedSection = m.sectionIndex
		m.step = stepGeometry
		m.inputValue = ""

		// Set input field based on section type
		switch selectedSection {
		case 0: // Rectangulaire plein -> b, h
			m.inputField = "b"
		case 1: // Circulaire plein -> r
			m.inputField = "r"
		case 2: // Circulaire creux -> r (rayon extérieur)
			m.inputField = "r_ext"
		case 3: // Rectangulaire creux -> b, h
			m.inputField = "b"
		case 4: // Poutre en I -> b, h
			m.inputField = "b"
		case 5: // Poutre en U -> b, h
			m.inputField = "b"
		}
	case stepGeometry:
		m.saveGeometryInput()
		m.inputValue = ""
	case stepMaterial:
		E = m.inputValue
		m.inputValue = ""
		m.step = stepNu
	case stepNu:
		nu = m.inputValue
		m.inputValue = ""
		beam := beams[selectedBeam]
		if beam.loadType == "point_load" {
			m.step = stepLoadP
		} else {
			m.step = stepLoadQ
		}
	case stepLoadP:
		if m.inputValue == "" {
			return m, nil
		}
		P = m.inputValue
		m.inputValue = ""
		m.step = stepLoadX
	case stepLoadX:
		xP = m.inputValue
		m.inputValue = ""
		m.step = stepSummary
	case stepLoadQ:
		if m.inputValue == "" {
			return m, nil
		}
		q = m.inputValue
		m.inputValue = ""
		m.step = stepLoadXEnd
	case stepLoadXEnd:
		if m.inputValue == "" {
			return m, nil
		}
		xEnd = m.inputValue
		m.inputValue = ""
		m.step = stepSummary
	case stepSummary:
		m.step = stepConfirm
	case stepConfirm:
		m.saveProblem()
		m.statusMsg = "Copie vers Kubernetes..."
		m.step = stepCopyToK8s
		return m, m.copyToK8s()
	}
	return m, nil
}

func (m *model) handleTab() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSelectBeam:
		m.beamIndex = (m.beamIndex + 1) % len(beams)
	case stepSection:
		m.sectionIndex = (m.sectionIndex + 1) % len(sectionCards)
	}
	return m, nil
}

func (m *model) handleRight() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSelectBeam:
		m.beamIndex = (m.beamIndex + 1) % len(beams)
	case stepSection:
		m.sectionIndex = (m.sectionIndex + 1) % len(sectionCards)
	}
	return m, nil
}

func (m *model) handleLeft() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSelectBeam:
		m.beamIndex = (m.beamIndex - 1 + len(beams)) % len(beams)
	case stepSection:
		m.sectionIndex = (m.sectionIndex - 1 + len(sectionCards)) % len(sectionCards)
	}
	return m, nil
}

func (m *model) handleDown() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSelectBeam:
		m.beamIndex = (m.beamIndex + 3) % len(beams)
	case stepSection:
		m.sectionIndex = (m.sectionIndex + 3) % len(sectionCards)
	}
	return m, nil
}

func (m *model) handleUp() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepSelectBeam:
		m.beamIndex = (m.beamIndex - 3 + len(beams)) % len(beams)
	case stepSection:
		m.sectionIndex = (m.sectionIndex - 3 + len(sectionCards)) % len(sectionCards)
	}
	return m, nil
}

func (m *model) saveGeometryInput() {
	switch m.inputField {
	case "b":
		b = m.inputValue
		m.inputField = "h"
	case "h":
		h = m.inputValue
		// For I, U and rectangular hollow, we need thickness after h
		if selectedSection == 3 {
			m.inputField = "t"
		} else if selectedSection == 4 || selectedSection == 5 {
			m.inputField = "tw"
		} else {
			m.step = stepMaterial
		}
	case "t":
		tw = m.inputValue // reuse tw for thickness
		m.step = stepMaterial
	case "r":
		r = m.inputValue
		m.step = stepMaterial
	case "r_ext":
		r = m.inputValue
		m.inputField = "r_int"
	case "r_int":
		r_int = m.inputValue
		m.step = stepMaterial
	case "tw":
		tw = m.inputValue
		m.inputField = "tf"
	case "tf":
		tf = m.inputValue
		m.step = stepMaterial
	}
}

func (m *model) saveLoadInput() {
	beam := beams[selectedBeam]
	if beam.loadType == "point_load" {
		switch m.inputField {
		case "load":
			P = m.inputValue
			m.inputField = "xP"
		case "xP":
			xP = m.inputValue
		}
	} else {
		switch m.inputField {
		case "load":
			q = m.inputValue
			m.inputField = "xEnd"
		case "xEnd":
			xEnd = m.inputValue
		}
	}
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (m *model) saveProblem() {
	beam := beams[selectedBeam]
	sec := sectionCards[selectedSection]

	I := "0"
	W := "0"
	hEquiv := "0"

	switch selectedSection {
	case 0: // Rectangulaire plein
		bf, _ := strconv.ParseFloat(b, 64)
		hf, _ := strconv.ParseFloat(h, 64)
		I = fmt.Sprintf("%f", bf*hf*hf*hf/12)
		W = fmt.Sprintf("%f", bf*hf*hf*hf/12/(hf/2))
		hEquiv = h
	case 1: // Circulaire plein
		rf, _ := strconv.ParseFloat(r, 64)
		I = fmt.Sprintf("%f", 3.1415926535*rf*rf*rf*rf/4)
		W = fmt.Sprintf("%f", 3.1415926535*rf*rf*rf*rf/4/rf)
		hEquiv = fmt.Sprintf("%f", 2*rf)
	case 2: // Circulaire creux (tube)
		re, _ := strconv.ParseFloat(r, 64)
		ri, _ := strconv.ParseFloat(r_int, 64)
		Ival := 3.1415926535 * (re*re*re*re - ri*ri*ri*ri) / 4
		I = fmt.Sprintf("%f", Ival)
		W = fmt.Sprintf("%f", Ival/re)
		hEquiv = fmt.Sprintf("%f", 2*re)
	case 3: // Rectangulaire creux
		bf, _ := strconv.ParseFloat(b, 64)
		hf, _ := strconv.ParseFloat(h, 64)
		I = fmt.Sprintf("%f", bf*hf*hf*hf/12)
		W = fmt.Sprintf("%f", bf*hf*hf/6)
		hEquiv = h
	case 4: // Poutre en I
		bf, _ := strconv.ParseFloat(b, 64)
		hf, _ := strconv.ParseFloat(h, 64)
		I = fmt.Sprintf("%f", bf*hf*hf*hf/12)
		W = fmt.Sprintf("%f", bf*hf*hf/6)
		hEquiv = h
	case 5: // Poutre en U
		bf, _ := strconv.ParseFloat(b, 64)
		hf, _ := strconv.ParseFloat(h, 64)
		I = fmt.Sprintf("%f", bf*hf*hf*hf/12)
		W = fmt.Sprintf("%f", bf*hf*hf/6)
		hEquiv = h
	}

	Ef, _ := strconv.ParseFloat(E, 64)
	Ei := int64(Ef * 1e9)

	PkN, _ := strconv.ParseFloat(P, 64)
	QkN, _ := strconv.ParseFloat(q, 64)

	problem := map[string]interface{}{
		"name":   beam.id,
		"length": length,
		"supports": map[string]string{
			"A": beam.appuiA,
			"B": beam.appuiB,
		},
		"section": map[string]interface{}{
			"type":    sec.id,
			"I":       I,
			"W":       W,
			"h_equiv": hEquiv,
			"b":       nullIfEmpty(b),
			"h":       nullIfEmpty(h),
			"r":       nullIfEmpty(r),
			"r_int":   nullIfEmpty(r_int),
			"tw":      nullIfEmpty(tw),
			"tf":      nullIfEmpty(tf),
		},
		"material": map[string]interface{}{
			"E":  Ei,
			"nu": nullIfEmpty(nu),
		},
		"load": map[string]interface{}{
			"type":    beam.loadType,
			"P":       fmt.Sprintf("%.0f", PkN*1000),
			"q":       nullIfEmpty(fmt.Sprintf("%.0f", QkN*1000)),
			"xP":      nullIfEmpty(xP),
			"x_start": "0",
			"x_end":   nullIfEmpty(xEnd),
		},
	}

	data, _ := json.MarshalIndent(problem, "", "  ")
	os.WriteFile("problem.json", data, 0644)
}

func copyToJupyter() error {
	cmd := exec.Command(
		"kubectl",
		"cp",
		"problem.json",
		"-n", "fenec",
		"jupyter-elfenec:/home/jovyan/problem.json",
		"-c", "notebook",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run()
}

func runNotebookCmd() error {
	// Run papermill directly (papermill is already installed in the image)
	cmd := exec.Command(
		"kubectl",
		"exec",
		"-n", "fenec",
		"jupyter-elfenec",
		"--",
		"papermill",
		"poutre.ipynb",
		"output.ipynb",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func getResultsCmd() (Result, error) {
	cmd := exec.Command(
		"kubectl",
		"cp",
		"-n", "fenec",
		"jupyter-elfenec:/home/jovyan/output.json",
		"output.json",
		"-c", "notebook",
	)

	if err := cmd.Run(); err != nil {
		return Result{}, err
	}

	data, err := os.ReadFile("output.json")
	if err != nil {
		return Result{}, err
	}

	fmt.Printf("Raw JSON: %s\n", string(data))

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return Result{}, err
	}

	fmt.Printf("Result: %+v\n", result)
	return result, nil
}

func (m *model) copyToK8s() tea.Cmd {
	return func() tea.Msg {
		fmt.Println("copyToK8s: starting")
		err := copyToJupyter()
		fmt.Println("copyToK8s: done, err=", err)
		return copyDoneMsg{err: err}
	}
}

func (m *model) runNotebook() tea.Cmd {
	return func() tea.Msg {
		err := runNotebookCmd()
		return notebookDoneMsg{err: err}
	}
}

func (m *model) getResults() tea.Cmd {
	return func() tea.Msg {
		result, err := getResultsCmd()
		return resultsDoneMsg{err: err, result: result}
	}
}

func (m *model) renderBeamCards() string {
	cards := make([]string, len(beams))
	for i, beam := range beams {
		num := fmt.Sprintf("[%d] %s", i+1, beam.title)

		borderColor := "3" // yellow for point load
		if beam.loadType == "distributed_load" {
			borderColor = "5" // purple for distributed load
		}

		card := lipgloss.NewStyle().
			Width(28).
			Height(9).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(borderColor))

		if m.beamIndex == i {
			focusedCard := lipgloss.NewStyle().
				Width(28).
				Height(9).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color(borderColor)).
				Foreground(lipgloss.Color(borderColor)).
				Bold(true)
			cards[i] = focusedCard.Render(num + "\n\n\n" + beam.schema)
		} else {
			cards[i] = card.Render(num + "\n\n\n" + beam.schema)
		}
	}

	rows := []string{}
	for i := 0; i < len(cards); i += 3 {
		end := i + 3
		if end > len(cards) {
			end = len(cards)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
	}
	return lipgloss.JoinVertical(lipgloss.Top, rows...)
}

func (m *model) renderSectionCards() string {
	cards := make([]string, len(sectionCards))

	for i, sec := range sectionCards {
		num := fmt.Sprintf("[%d] %s", i+1, sec.title)

		card := lipgloss.NewStyle().
			Width(22).
			Height(14).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("69"))

		if m.sectionIndex == i {
			focusedCard := lipgloss.NewStyle().
				Width(22).
				Height(14).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69")).
				Foreground(lipgloss.Color("69")).
				Bold(true)
			cards[i] = focusedCard.Render(num + "\n" + sec.schema)
		} else {
			cards[i] = card.Render(num + "\n" + sec.schema)
		}
	}

	rows := []string{}
	for i := 0; i < len(cards); i += 3 {
		end := i + 3
		if end > len(cards) {
			end = len(cards)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
	}
	return lipgloss.JoinVertical(lipgloss.Top, rows...)
}

func (m *model) View() string {
	var s strings.Builder

	switch m.step {
	case stepSelectBeam:
		s.WriteString(headerStyle.Render("Sélectionnez le type de poutre") + "\n\n")
		s.WriteString(m.renderBeamCards())

		legend := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"\n\nLégende: " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("■ Charge ponctuelle") +
				"    " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("■ Charge répartie"))
		s.WriteString(legend)
		s.WriteString(helpStyle.Render("\n\n↑↓/Tab: naviguer  • Entrée: sélectionner  • Q: quitter"))

	case stepLength:
		beam := beams[selectedBeam]
		s.WriteString(headerStyle.Render("Longueur de la poutre") + "\n\n")
		s.WriteString(fmt.Sprintf("Type sélectionné: %s\n", inputStyle.Render(beam.title)))
		s.WriteString(fmt.Sprintf("\nLongueur L (m): %s", inputStyle.Render(m.inputValue+"_")))
		s.WriteString(helpStyle.Render("\n\nEntrée: suivant  • Q: quitter"))

	case stepSection:
		s.WriteString(headerStyle.Render("Géométrie - Section") + "\n\n")
		s.WriteString(m.renderSectionCards())
		s.WriteString(helpStyle.Render("\n←→: sélectionner  • Entrée: suivant  • Q: quitter"))

	case stepGeometry:
		s.WriteString(headerStyle.Render("Géométrie - Dimensions") + "\n\n")

		sectionCardsHTML := m.renderSectionCards()
		s.WriteString(sectionCardsHTML)

		// Show input field based on section type
		switch selectedSection {
		case 0: // Rectangulaire plein
			if m.inputField == "b" {
				s.WriteString(fmt.Sprintf("\n\nLargeur b (m): %s", inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "h" {
				s.WriteString(fmt.Sprintf("\n\nLargeur b: %sm\nHauteur h (m): %s", b, inputStyle.Render(m.inputValue+"_")))
			}
		case 1: // Circulaire plein
			s.WriteString(fmt.Sprintf("\n\nRayon r (m): %s", inputStyle.Render(m.inputValue+"_")))
		case 2: // Circulaire creux
			if m.inputField == "r_ext" {
				s.WriteString(fmt.Sprintf("\n\nRayon extérieur Re (m): %s", inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "r_int" {
				s.WriteString(fmt.Sprintf("\n\nRayon extérieur: %sm\nRayon intérieur Ri (m): %s", r, inputStyle.Render(m.inputValue+"_")))
			}
		case 3: // Rectangulaire creux
			if m.inputField == "b" {
				s.WriteString(fmt.Sprintf("\n\nLargeur extérieure b (m): %s", inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "h" {
				s.WriteString(fmt.Sprintf("\n\nLargeur ext: %sm\nHauteur extérieure h (m): %s", b, inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "t" {
				s.WriteString(fmt.Sprintf("\n\nb=%sm, h=%sm\nÉpaisseur t (m): %s", b, h, inputStyle.Render(m.inputValue+"_")))
			}
		case 4: // Poutre en I
			if m.inputField == "b" {
				s.WriteString(fmt.Sprintf("\n\nLargeur totale b (m): %s", inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "h" {
				s.WriteString(fmt.Sprintf("\n\nLargeur: %sm\nHauteur totale h (m): %s", b, inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "tw" {
				s.WriteString(fmt.Sprintf("\n\nb=%sm, h=%sm\nÉpaisseur âme tw (m): %s", b, h, inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "tf" {
				s.WriteString(fmt.Sprintf("\n\nb=%sm, h=%sm, tw=%sm\nÉpaisseur semelle tf (m): %s", b, h, tw, inputStyle.Render(m.inputValue+"_")))
			}
		case 5: // Poutre en U
			if m.inputField == "b" {
				s.WriteString(fmt.Sprintf("\n\nLargeur totale b (m): %s", inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "h" {
				s.WriteString(fmt.Sprintf("\n\nLargeur: %sm\nHauteur totale h (m): %s", b, inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "tw" {
				s.WriteString(fmt.Sprintf("\n\nb=%sm, h=%sm\nÉpaisseur âme tw (m): %s", b, h, inputStyle.Render(m.inputValue+"_")))
			} else if m.inputField == "tf" {
				s.WriteString(fmt.Sprintf("\n\nb=%sm, h=%sm, tw=%sm\nÉpaisseur semelle tf (m): %s", b, h, tw, inputStyle.Render(m.inputValue+"_")))
			}
		}

		s.WriteString(helpStyle.Render("\n←→: sélectionner  • Entrée: suivant  • Q: quitter"))

	case stepMaterial:
		s.WriteString(headerStyle.Render("Matériau") + "\n\n")
		s.WriteString(inputStyle.Render(fmt.Sprintf("Module d'Young E (GPa): %s", m.inputValue+"_")))
		s.WriteString(helpStyle.Render("\n\nEntrée: suivant  • Q: quitter"))

	case stepNu:
		s.WriteString(inputStyle.Render(fmt.Sprintf("Coefficient de Poisson nu (optionnel): %s", m.inputValue+"_")))
		s.WriteString(helpStyle.Render("\n\nEntrée: suivant (laisser vide si non utilisé)  • Q: quitter"))

	case stepLoadP:
		s.WriteString(headerStyle.Render("Charge") + "\n\n")
		s.WriteString(inputStyle.Render(fmt.Sprintf("Force P (kN): %s", m.inputValue+"_")))
		s.WriteString(helpStyle.Render("\n\nEntrée: suivant  • Q: quitter"))

	case stepLoadX:
		s.WriteString(inputStyle.Render(fmt.Sprintf("Position x (m): %s", m.inputValue+"_")))
		s.WriteString(helpStyle.Render("\n\nEntrée: suivant  • Q: quitter"))

	case stepLoadQ:
		s.WriteString(inputStyle.Render(fmt.Sprintf("Charge q (kN/m): %s", m.inputValue+"_")))
		s.WriteString(helpStyle.Render("\n\nEntrée: suivant  • Q: quitter"))

	case stepLoadXEnd:
		s.WriteString(inputStyle.Render(fmt.Sprintf("Position finale (m): %s", m.inputValue+"_")))
		s.WriteString(helpStyle.Render("\n\nEntrée: suivant  • Q: quitter"))

	case stepSummary:
		beam := beams[selectedBeam]
		sec := sectionCards[selectedSection]
		secDim := ""
		switch selectedSection {
		case 0: // Rectangulaire plein
			secDim = fmt.Sprintf("b=%sm, h=%sm", b, h)
		case 1: // Circulaire plein
			secDim = fmt.Sprintf("r=%sm", r)
		case 2: // Circulaire creux
			secDim = fmt.Sprintf("Re=%sm, Ri=%sm", r, r_int)
		case 3: // Rectangulaire creux
			secDim = fmt.Sprintf("b=%sm, h=%sm, t=%sm", b, h, tw)
		case 4: // Poutre en I
			secDim = fmt.Sprintf("b=%sm, h=%sm, tw=%sm, tf=%sm", b, h, tw, tf)
		case 5: // Poutre en U
			secDim = fmt.Sprintf("b=%sm, h=%sm, tw=%sm, tf=%sm", b, h, tw, tf)
		}
		loadInfo := ""
		if beam.loadType == "point_load" {
			if P != "" && xP != "" {
				loadInfo = fmt.Sprintf("P=%skN à x=%sm", P, xP)
			} else {
				loadInfo = "Non défini"
			}
		} else {
			if q != "" && xEnd != "" {
				loadInfo = fmt.Sprintf("q=%skN/m sur [0, %sm]", q, xEnd)
			} else {
				loadInfo = "Non défini"
			}
		}

		summary := fmt.Sprintf(`%s
Appuis: %s | %s
Longueur: %sm
Section: %s
Dimensions: %s
Module E: %s GPa
Charge: %s`, beam.title, beam.appuiA, beam.appuiB, length, sec.title, secDim, E, loadInfo)

		summaryStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			BorderForeground(lipgloss.Color("212")).
			Border(lipgloss.DoubleBorder()).
			Align(lipgloss.Center).
			Width(50).
			Margin(1, 2).
			Padding(2, 4)

		s.WriteString(headerStyle.Render("Résumé") + "\n\n")
		s.WriteString(summaryStyle.Render(summary))
		s.WriteString(helpStyle.Render("\n\nEntrée: confirmer  • Q: quitter"))

	case stepConfirm:
		s.WriteString(confirmStyle.Render("✓ problem.json généré avec succès!"))
		s.WriteString(helpStyle.Render("\n\nEntrée: exécuter  • Q: quitter"))

	case stepCopyToK8s:
		s.WriteString(headerStyle.Render("Copie vers Kubernetes") + "\n\n")
		s.WriteString(m.spinner.View() + "\n")
		s.WriteString(inputStyle.Render(m.statusMsg))
		s.WriteString(helpStyle.Render("\n\nQ: quitter"))

	case stepRunNotebook:
		s.WriteString(headerStyle.Render("Exécution du notebook") + "\n\n")
		s.WriteString(m.spinner.View() + "\n")
		s.WriteString(inputStyle.Render(m.statusMsg))
		s.WriteString(helpStyle.Render("\n\nQ: quitter"))

	case stepDone:
		r := m.result
		statusColor := "70"
		if !r.StatusGlobal {
			statusColor = "196"
		}
		statusStr := "OK"
		if !r.StatusGlobal {
			statusStr = "ÉCHEC"
		}
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Bold(true)

		arrow := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).Render("▸")

		sigmaMax := fmt.Sprintf("%.2f", r.Contrainte.SigmaMaxMPa)
		sigmaLim := fmt.Sprintf("%.2f", r.Contrainte.LimiteMPa)
		coefSec := fmt.Sprintf("%.2f", r.Contrainte.CoefficientSecurite)
		okContrainte := "✓"
		if !r.Contrainte.OK {
			okContrainte = "✗"
		}

		deltaMax := fmt.Sprintf("%.2f", r.Fleche.DeltaMaxMM)
		deltaLim := fmt.Sprintf("%.2f", r.Fleche.DeltaLimMM)
		okFleche := "✓"
		if !r.Fleche.OK {
			okFleche = "✗"
		}

		analytique := fmt.Sprintf("%.4f", r.Comparaison.AnalytiqueMM)
		fem := fmt.Sprintf("%.4f", r.Comparaison.FemMM)
		erreur := fmt.Sprintf("%.2f", r.Comparaison.ErreurPourcent)

		resultView := fmt.Sprintf("Vérification globale: %s\n\n%sContrainte:\n  σmax: %s MPa\n  σlimite: %s MPa\n  Coefficient sécurité: %s\n  OK: %s\n\n%sFlèche:\n  δmax: %s mm\n  δlimite: %s mm\n  OK: %s\n\n%sComparaison:\n  Analytique: %s mm\n  FEM: %s mm\n  Erreur: %s %%",
			statusStyle.Render(statusStr),
			arrow,
			sigmaMax,
			sigmaLim,
			coefSec,
			okContrainte,
			arrow,
			deltaMax,
			deltaLim,
			okFleche,
			arrow,
			analytique,
			fem,
			erreur,
		)

		resultBox := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(1, 2).
			Width(45)

		s.WriteString(headerStyle.Render("Résultats") + "\n\n")
		s.WriteString(resultBox.Render(resultView))
		s.WriteString(helpStyle.Render("\n\nQ: quitter"))
	}

	return s.String()
}

var (
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true).Padding(1)
	confirmStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("70")).Bold(true).Padding(1)
	cardStyle    = lipgloss.NewStyle().
			Width(28).
			Height(9).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
	focusedCardStyle = lipgloss.NewStyle().
				Width(28).
				Height(9).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69")).
				Foreground(lipgloss.Color("69"))
	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69")).
			Bold(true)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func main() {
	p := tea.NewProgram(
		newModel(),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
