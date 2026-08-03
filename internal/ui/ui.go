package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	colorPurple = "\033[38;5;129m"
	colorYellow = "\033[38;5;226m"
	colorRed    = "\033[38;5;196m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
	colorReset  = "\033[0m"

	clrInfo  = "\033[38;5;135m"
	clrOk    = "\033[38;5;220m"
	clrWarn  = "\033[38;5;226m"
	clrErr   = "\033[1;38;5;196m"
	clrMeta  = "\033[38;5;93m"
	clrAcct  = "\033[38;5;214m"
	clrDim   = "\033[2m"
	clrBold  = "\033[1m"
	clrReset = "\033[0m"
)

const (
	ClrOk     = "\033[38;5;220m"
	ClrWarn   = "\033[38;5;226m"
	ClrErr    = "\033[1;38;5;196m"
	ClrInfo   = "\033[38;5;135m"
	ClrMeta   = "\033[38;5;93m"
	ClrDimStr = "\033[2m"
	ClrReset  = "\033[0m"
)

const RepoURL = "https://github.com/z1rov/z1"

var greens = []string{"93", "99", "129", "135", "141", "213"}

func Info(msg string) {
	fmt.Printf("  %s[*]%s %s\n", clrInfo, clrReset, msg)
}

func Ok(msg string) {
	fmt.Printf("  %s[+]%s %s\n", clrOk, clrReset, msg)
}

func Error(msg string) {
	fmt.Fprintf(os.Stderr, "  %s[!]%s %s\n", clrErr, clrReset, msg)
}

func Warn(msg string) {
	fmt.Printf("  %s[~]%s %s\n", clrWarn, clrReset, msg)
}

func Dim(msg string) {
	fmt.Printf("  %s%s%s\n", clrDim, msg, clrReset)
}

func Bold(msg string) {
	fmt.Printf("  %s%s%s\n", clrBold, msg, clrReset)
}

func Blank() {
	fmt.Println()
}

func Divider() {
	fmt.Printf("  %s%s%s\n", clrDim, strings.Repeat("─", 48), clrReset)
}

func KV(key, value, valueColor string) {
	fmt.Printf("  %s%-10s%s %s·%s  %s%s%s\n",
		clrDim, key, clrReset,
		clrDim, clrReset,
		valueColor, value, clrReset,
	)
}

func printAsciiArt() {
	artLines := []string{
		"     ███████╗ ██╗",
		"     ╚══███╔╝███║",
		"       ███╔╝ ╚██║",
		"      ███╔╝   ██║",
		"     ███████╗ ██║",
		"     ╚══════╝ ╚═╝",
	}
	maxWidth := 0
	for _, line := range artLines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}
	for i, line := range artLines {
		color := greens[i%len(greens)]
		fmt.Printf("  \033[38;5;%sm%-*s\033[0m\n", color, maxWidth, line)
	}
}

func printMeta() {
	fmt.Printf("  %s[Meta]%s Created by z1rov\n", clrMeta, clrReset)
	fmt.Printf("  %s[Meta]%s %shttps://www.zirov.net%s\n", clrMeta, clrReset, clrDim, clrReset)
	fmt.Printf("  %s[Meta]%s %s%s%s\n", clrMeta, clrReset, clrDim, RepoURL, clrReset)
}

func Banner() {
	fmt.Println()
	printAsciiArt()
	fmt.Println()
	printMeta()
	fmt.Println()
}

func StartScreen(anvil string) {
	fmt.Println()
	printAsciiArt()

	fmt.Printf("        ")
	for _, ch := range "Isolated" {
		fmt.Printf("\033[38;5;226m%c\033[0m", ch)
		time.Sleep(80 * time.Millisecond)
	}
	fmt.Printf("\n\n")

	printMeta()
	fmt.Println()
	fmt.Printf("  %s[Info]%s Initializing container services:\n", clrInfo, clrReset)
	fmt.Printf("  %s[%-13s]%s%s::Network    host%s\n", clrInfo, "host", clrReset, clrDim, clrReset)
	fmt.Printf("  %s[%-13s]%s%s::Mount      /anvil → %s%s\n", clrAcct, "mount", clrReset, clrDim, anvil, clrReset)
	fmt.Println()
	fmt.Printf("  %s[Info]%s %sWelcome! Good luck with your pentesting ;)%s\n", clrInfo, clrReset, clrBold, clrReset)
	Divider()
	fmt.Println()
}

func GoodbyeScreen() {
	fmt.Println()
	fmt.Printf("  %s[z1]%s %sSession ended.%s\n", clrInfo, clrReset, clrBold, clrReset)
	fmt.Printf("  %s[*]%s Hope the hunt was good. Stay safe out there.\n", clrDim, clrReset)
	fmt.Println()
}

type Spinner struct {
	label string
	stop  chan struct{}
	wg    sync.WaitGroup
}

func NewSpinner(label string) *Spinner {
	s := &Spinner{
		label: label,
		stop:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.run()
	return s
}

func (s *Spinner) run() {
	defer s.wg.Done()
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-s.stop:
			fmt.Printf("\r\033[K")
			return
		default:
			fmt.Printf("\r  %s%s%s  %s%s%s",
				clrInfo, frames[i%len(frames)], clrReset,
				clrDim, s.label, clrReset)
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

func (s *Spinner) Stop() {
	close(s.stop)
	s.wg.Wait()
}

func LayerLine(id, status string) {
	color := clrDim
	icon := "·"

	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "pull complete"), strings.Contains(lower, "already exists"):
		color = clrOk
		icon = "✔"
	case strings.Contains(lower, "pulling"):
		color = colorPurple
		icon = "↓"
	case strings.Contains(lower, "extract"):
		color = colorYellow
		icon = "⧗"
	case strings.Contains(lower, "verif"):
		color = clrMeta
		icon = "◈"
	}

	short := id
	if len(id) > 12 {
		short = id[:12]
	}
	fmt.Printf("  %s%s  %-14s  %s%s\n", color, icon, short, status, clrReset)
}

func Usage(imageInstalled, containerRunning bool) {
	fmt.Println()
	printAsciiArt()
	fmt.Printf("        \033[38;5;226mIsolated\033[0m\n\n")

	printMeta()
	fmt.Println()
	fmt.Printf("  %s[Info]%s Usage: z1 <command>\n", clrInfo, clrReset)
	fmt.Println()

	imgState, imgColor := "not installed", clrErr
	if imageInstalled {
		imgState, imgColor = "installed", clrOk
	}
	ctrState, ctrColor := "not running", clrWarn
	if containerRunning {
		ctrState, ctrColor = "running", clrOk
	}
	fmt.Printf("  %s[Status]%s image %s%s%s  ·  container %s%s%s\n",
		clrMeta, clrReset, imgColor, imgState, clrReset, ctrColor, ctrState, clrReset)
	fmt.Println()

	type entry struct{ cmd, desc string }
	groups := []struct {
		title   string
		color   string
		entries []entry
	}{
		{
			"container", "196",
			[]entry{
				{"start", "start z1 container"},
				{"stop", "stop z1 container"},
				{"status", "show container status"},
				{"logs", "show container logs"},
				{"logs -f", "follow container logs"},
				{"exec <cmd>", "run command in container"},
			},
		},
		{
			"image", "93",
			[]entry{
				{"install", "pull z1 image"},
				{"update", "update z1 image"},
				{"delete", "remove all images & containers"},
				{"version", "show version info"},
			},
		},
		{
			"system", "214",
			[]entry{
				{"relocate", "move Docker data-root to ~/docker-data"},
				{"synctime <dc-ip>", "sync clock with a DC (Kerberos)"},
				{"synctime restore", "re-enable host NTP sync"},
			},
		},
		{
			"general", "226",
			[]entry{
				{"help", "show this help"},
			},
		},
	}

	cmdWidth := 0
	for _, g := range groups {
		for _, e := range g.entries {
			if len(e.cmd) > cmdWidth {
				cmdWidth = len(e.cmd)
			}
		}
	}
	cmdWidth++

	for _, g := range groups {
		for _, e := range g.entries {
			fmt.Printf("  \033[38;5;%sm[%-13s]\033[0m%s::%s %s%-*s%s %s%s%s\n",
				g.color, g.title,
				clrDim, clrReset,
				clrBold, cmdWidth, e.cmd, clrReset,
				clrDim, e.desc, clrReset)
		}
	}

	fmt.Println()
	Divider()
	fmt.Println()
}

func VersionScreen(local string, localOk bool, remote string, remoteOk bool) {
	Banner()
	fmt.Printf("  %s[Info]%s Checking version:\n", clrInfo, clrReset)
	fmt.Println()

	localVal, localColor := local, clrOk
	if !localOk {
		localVal, localColor = "not installed", clrErr
	}
	fmt.Printf("  %s[%-13s]%s%s::%s %s%s%s\n",
		clrAcct, "local", clrReset, clrDim, clrReset, localColor, localVal, clrReset)

	remoteVal, remoteColor := remote, clrWarn
	if !remoteOk {
		remoteVal, remoteColor = "unavailable", clrErr
	}
	fmt.Printf("  %s[%-13s]%s%s::%s %s%s%s\n",
		clrMeta, "remote", clrReset, clrDim, clrReset, remoteColor, remoteVal, clrReset)

	fmt.Println()

	if localOk && remoteOk {
		if local == remote {
			fmt.Printf("  %s[Info]%s %sup to date%s\n", clrInfo, clrReset, clrBold, clrReset)
		} else {
			fmt.Printf("  %s[Warn]%s %supdate available: %s → %s — run: z1 update%s\n",
				clrWarn, clrReset, clrBold, local, remote, clrReset)
		}
	}

	fmt.Println()
	Divider()
	fmt.Println()
}

func StartHeader() {
	fmt.Printf("  %s[+]%s %sstarting z1 container%s\n", clrOk, clrReset, clrBold, clrReset)
}

func StartDetail(label, value string) {
	fmt.Printf("  %s[·]%s %s%-10s%s %s%s%s\n",
		clrDim, clrReset, clrDim, label+":", clrReset, colorPurple, value, clrReset)
}

func StartDone() {
	fmt.Printf("  %s[✔]%s %scontainer ready%s\n", clrOk, clrReset, clrBold, clrReset)
}

func StopHeader() {
	fmt.Printf("  %s[~]%s %sstopping z1 container%s\n", clrWarn, clrReset, clrBold, clrReset)
}

func StopDone() {
	fmt.Printf("  %s[+]%s %scontainer stopped%s\n", clrOk, clrReset, clrBold, clrReset)
}

func StorageStep(msg string) {
	fmt.Printf("\n  %s[·]%s %s%s%s\n", clrMeta, clrReset, clrBold, msg, clrReset)
}

func StorageOk(msg string) {
	fmt.Printf("  %s[✔]%s %s\n", clrOk, clrReset, msg)
}

func StorageWarn(msg string) {
	fmt.Printf("  %s[~]%s %s\n", clrWarn, clrReset, msg)
}

func StorageErr(msg string) {
	fmt.Fprintf(os.Stderr, "  %s[!]%s %s\n", clrErr, clrReset, msg)
}

func StorageKV(label, value string) {
	fmt.Printf("  %s  %-18s%s %s\n", clrDim, label, clrReset, value)
}
