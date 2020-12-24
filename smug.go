package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		return strings.Replace(path, "~", userHome, 1)
	}

	return path
}

func Contains(slice []string, s string) bool {
	for _, e := range slice {
		if e == s {
			return true
		}
	}

	return false
}

type Smug struct {
	tmux      Tmux
	commander Commander
}

func (smug Smug) execShellCommands(commands []string, path string) error {
	for _, c := range commands {

		cmd := exec.Command("/bin/sh", "-c", c)
		cmd.Dir = path

		_, err := smug.commander.Exec(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (smug Smug) switchOrAttach(ses string, windows []string, attach bool) error {
	insideTmuxSession := os.Getenv("TERM") == "screen"
	if insideTmuxSession && attach {
		return smug.tmux.SwitchClient(ses)
	} else if !insideTmuxSession {
		return smug.tmux.Attach(ses, os.Stdin, os.Stdout, os.Stderr)
	}
	return nil
}

func (smug Smug) Stop(config Config, windows []string) error {
	if len(windows) == 0 {

		sessionRoot := ExpandPath(config.Root)

		err := smug.execShellCommands(config.Stop, sessionRoot)
		if err != nil {
			return err
		}
		_, err = smug.tmux.StopSession(config.Session)
		return err
	}

	for _, w := range windows {
		err := smug.tmux.KillWindow(config.Session + ":" + w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (smug Smug) Start(config Config, windows []string, attach bool) error {
	var ses string
	var err error

	sessionRoot := ExpandPath(config.Root)

	sessionExists := smug.tmux.SessionExists(config.Session)
	if !sessionExists {
		err = smug.execShellCommands(config.BeforeStart, sessionRoot)
		if err != nil {
			return err
		}

		var defaultWindowName string
		if len(windows) > 0 {
			defaultWindowName = windows[0]
		} else if len(config.Windows) > 0 {
			defaultWindowName = config.Windows[0].Name
		}

		ses, err = smug.tmux.NewSession(config.Session, sessionRoot, defaultWindowName)
		if err != nil {
			return err
		}
	} else {
		ses = config.Session + ":"
		if len(windows) == 0 {
			smug.switchOrAttach(ses, windows, attach)
			return nil
		}
	}

	for wIndex, w := range config.Windows {
		if (len(windows) == 0 && w.Manual) || (len(windows) > 0 && !Contains(windows, w.Name)) {
			continue
		}

		windowRoot := ExpandPath(w.Root)
		if windowRoot == "" || !filepath.IsAbs(windowRoot) {
			windowRoot = filepath.Join(sessionRoot, w.Root)
		}

		window := ses + w.Name
		if (!sessionExists && wIndex > 0 && len(windows) == 0) || (sessionExists && len(windows) > 0) {
			_, err = smug.tmux.NewWindow(ses, w.Name, windowRoot)
			if err != nil {
				return err
			}
		}

		for _, c := range w.Commands {
			err = smug.tmux.SendKeys(window, c)
			if err != nil {
				return err
			}
		}

		for _, p := range w.Panes {
			paneRoot := ExpandPath(p.Root)
			if paneRoot == "" || !filepath.IsAbs(p.Root) {
				paneRoot = filepath.Join(windowRoot, p.Root)
			}

			_, err = smug.tmux.SplitWindow(window, p.Type, paneRoot, p.Commands)
			if err != nil {
				return err
			}
		}

		layout := w.Layout
		if layout == "" {
			layout = EvenHorizontal
		}

		_, err = smug.tmux.SelectLayout(ses+w.Name, layout)
		if err != nil {
			return err
		}
	}

	if len(windows) == 0 {
		smug.switchOrAttach(ses, windows, attach)
	}

	return nil
}
