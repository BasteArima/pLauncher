package launcher

import "golang.org/x/sys/windows"

// ShellExecute uses file associations without interpreting filenames as cmd.exe syntax.
func openAssociated(path, folder string) error {
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(folder)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, nil, file, nil, cwd, 1)
}
