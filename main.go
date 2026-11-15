package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Panel struct {
	List  *tview.List
	Path  string
	Items []string
}

type FileManager struct {
	App       *tview.Application
	Root      *tview.Flex
	Left      *Panel
	Right     *Panel
	Active    int
	Status    *tview.TextView
	ModalOpen bool
}

// Create panel
func NewPanel(path string, status *tview.TextView) *Panel {
	p := &Panel{
		List: tview.NewList(),
		Path: path,
	}

	p.List.ShowSecondaryText(false)
	p.List.SetBorder(true)

	p.List.SetSelectedFunc(func(index int, main, secondary string, shortcut rune) {
		if index == 0 {
			parent := filepath.Dir(p.Path)
			if parent != p.Path {
				p.Path = parent
				p.Refresh()
			}
			return
		}

		if index >= len(p.Items) {
			return
		}

		path := filepath.Join(p.Path, p.Items[index])
		info, err := os.Stat(path)

		if err != nil {
			status.SetText(" Error: " + err.Error())
			return
		}

		if info.IsDir() {
			p.Path = path
			p.Refresh()
		} else {
			status.SetText(" Selected: " + path)
		}
	})

	p.Refresh()
	return p
}

// Refresh panel
func (p *Panel) Refresh() {
	p.List.Clear()
	p.Items = []string{".."}

	p.List.SetTitle(" " + p.Path + " ")

	p.List.AddItem("..", "", 0, nil)

	entries, err := os.ReadDir(p.Path)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		label := name

		if entry.IsDir() {
			label = "[DIR] " + name
		}

		p.Items = append(p.Items, name)
		p.List.AddItem(label, "", 0, nil)
	}
}

// Get selected file path
func (p *Panel) SelectedPath() string {
	index := p.List.GetCurrentItem()

	if index <= 0 || index >= len(p.Items) {
		return ""
	}

	return filepath.Join(p.Path, p.Items[index])
}

// Get active panel
func (fm *FileManager) ActivePanel() *Panel {
	if fm.Active == 0 {
		return fm.Left
	}
	return fm.Right
}

// Get inactive panel
func (fm *FileManager) OtherPanel() *Panel {
	if fm.Active == 0 {
		return fm.Right
	}
	return fm.Left
}

// Update active panel
func (fm *FileManager) UpdateFocus() {
	if fm.Active == 0 {
		fm.App.SetFocus(fm.Left.List)
		fm.Left.List.SetBorderColor(tcell.ColorYellow)
		fm.Right.List.SetBorderColor(tcell.ColorWhite)
	} else {
		fm.App.SetFocus(fm.Right.List)
		fm.Right.List.SetBorderColor(tcell.ColorWhite)
		fm.Left.List.SetBorderColor(tcell.ColorWhite)
		fm.Right.List.SetBorderColor(tcell.ColorYellow)
	}
}

// Copy file or directory recursively
func CopyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links are not supported yet")
	}

	if info.IsDir() {
		if err := os.Mkdir(dst, info.Mode().Perm()); err != nil {
			return err
		}

		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			srcPath := filepath.Join(src, entry.Name())
			dstPath := filepath.Join(dst, entry.Name())

			if err := CopyPath(srcPath, dstPath); err != nil {
				return err
			}
		}

		return nil
	}

	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()

	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}

	return os.Chmod(dst, info.Mode().Perm())
}

// Check destination
func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Copy selected item
func (fm *FileManager) Copy() {
	src := fm.ActivePanel().SelectedPath()

	if src == "" {
		fm.Status.SetText(" Nothing selected")
		return
	}

	dst := filepath.Join(
		fm.OtherPanel().Path,
		filepath.Base(src),
	)

	if Exists(dst) {
		fm.Status.SetText(" Destination already exists: " + dst)
		return
	}

	err := CopyPath(src, dst)
	if err != nil {
		fm.Status.SetText(" Copy error: " + err.Error())
		return
	}

	fm.OtherPanel().Refresh()
	fm.Status.SetText(" Copied: " + filepath.Base(src))
}

// Move selected item
func (fm *FileManager) Move() {
	src := fm.ActivePanel().SelectedPath()

	if src == "" {
		fm.Status.SetText(" Nothing selected")
		return
	}

	dst := filepath.Join(
		fm.OtherPanel().Path,
		filepath.Base(src),
	)

	if Exists(dst) {
		fm.Status.SetText(" Destination already exists")
		return
	}

	err := os.Rename(src, dst)

	// Cross-filesystem fallback
	if err != nil {
		err = CopyPath(src, dst)
		if err == nil {
			err = os.RemoveAll(src)
			if err != nil {
				os.RemoveAll(dst)
			}
		}
	}

	if err != nil {
		fm.Status.SetText(" Move error: " + err.Error())
		return
	}

	fm.ActivePanel().Refresh()
	fm.OtherPanel().Refresh()
	fm.Status.SetText(" Moved: " + filepath.Base(src))
}

// Open input dialog
func (fm *FileManager) InputDialog(
	title string,
	initial string,
	callback func(string),
) {
	input := tview.NewInputField().
		SetLabel(title).
		SetText(initial).
		SetFieldWidth(0)

	fm.ModalOpen = true
	fm.App.SetRoot(input, true)

	input.SetDoneFunc(func(key tcell.Key) {
		value := input.GetText()

		fm.ModalOpen = false
		fm.App.SetRoot(fm.Root, true)
		fm.UpdateFocus()

		if key == tcell.KeyEnter && value != "" {
			callback(value)
		}
	})

	fm.App.SetFocus(input)
}

// Create directory
func (fm *FileManager) Mkdir() {
	fm.InputDialog("New directory: ", "", func(name string) {
		path := filepath.Join(fm.ActivePanel().Path, name)

		err := os.Mkdir(path, 0755)
		if err != nil {
			fm.Status.SetText(" Error: " + err.Error())
			return
		}

		fm.ActivePanel().Refresh()
		fm.Status.SetText(" Directory created: " + name)
	})
}

// Rename selected item
func (fm *FileManager) Rename() {
	src := fm.ActivePanel().SelectedPath()

	if src == "" {
		return
	}

	oldName := filepath.Base(src)

	fm.InputDialog("Rename: ", oldName, func(newName string) {
		dst := filepath.Join(fm.ActivePanel().Path, newName)

		if Exists(dst) {
			fm.Status.SetText(" Destination already exists")
			return
		}

		err := os.Rename(src, dst)
		if err != nil {
			fm.Status.SetText(" Rename error: " + err.Error())
			return
		}

		fm.ActivePanel().Refresh()
		fm.Status.SetText(" Renamed: " + newName)
	})
}

// Delete confirmation
func (fm *FileManager) Delete() {
	path := fm.ActivePanel().SelectedPath()

	if path == "" {
		return
	}

	modal := tview.NewModal().
		SetText("Delete " + filepath.Base(path) + "?").
		AddButtons([]string{"Delete", "Cancel"})

	fm.ModalOpen = true
	fm.App.SetRoot(modal, true)

	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		fm.ModalOpen = false
		fm.App.SetRoot(fm.Root, true)
		fm.UpdateFocus()

		if buttonIndex != 0 {
			return
		}

		err := os.RemoveAll(path)
		if err != nil {
			fm.Status.SetText(" Delete error: " + err.Error())
			return
		}

		fm.ActivePanel().Refresh()
		fm.Status.SetText(" Deleted: " + filepath.Base(path))
	})
}
func (fm *FileManager) Keyboard(event *tcell.EventKey) *tcell.EventKey {
	if fm.ModalOpen {
		return event
	}

	switch event.Key() {
	case tcell.KeyTAB:
		fm.Active = 1 - fm.Active
		fm.UpdateFocus()
		return nil

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		panel := fm.ActivePanel()
		parent := filepath.Dir(panel.Path)

		if parent != panel.Path {
			panel.Path = parent
			panel.Refresh()
		}
		return nil

	case tcell.KeyF2:
		fm.Rename()
		return nil

	case tcell.KeyF5:
		fm.Copy()
		return nil

	case tcell.KeyF6:
		fm.Move()
		return nil

	case tcell.KeyF7:
		fm.Mkdir()
		return nil

	case tcell.KeyF8:
		fm.Delete()
		return nil
	}

	if event.Rune() == 'q' {
		fm.App.Stop()
		return nil
	}

	return event
}

// Resolve initial directory
func ResolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}

	return abs, nil
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	leftPath := cwd
	rightPath := cwd

	if len(os.Args) > 1 {
		leftPath = os.Args[1]
	}

	if len(os.Args) > 2 {
		rightPath = os.Args[2]
	}

	leftPath, err = ResolvePath(leftPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rightPath, err = ResolvePath(rightPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	app := tview.NewApplication()

	status := tview.NewTextView().
		SetText(" Tab: Switch | F2: Rename | F5: Copy | F6: Move | F7: Mkdir | F8: Delete | q: Quit")

	left := NewPanel(leftPath, status)
	right := NewPanel(rightPath, status)

	panels := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(left.List, 0, 1, true).
		AddItem(right.List, 0, 1, false)

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(panels, 0, 1, true).
		AddItem(status, 1, 0, false)

	fm := &FileManager{
		App:    app,
		Root:   root,
		Left:   left,
		Right:  right,
		Active: 0,
		Status: status,
	}

	left.List.SetBorderColor(tcell.ColorYellow)
	right.List.SetBorderColor(tcell.ColorWhite)

	app.SetInputCapture(fm.Keyboard)

	if err := app.SetRoot(root, true).
		SetFocus(left.List).
		Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
