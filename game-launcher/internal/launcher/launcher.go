package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Controller отвечает за взаимодействие с файловой системой ОС и процессами
type Controller struct {
	// command is injectable for tests so OS document openers never run there.
	command   func(string, ...string) *exec.Cmd
	associate func(string, string) error
}

func (c *Controller) cmd(name string, args ...string) *exec.Cmd {
	if c.command != nil {
		return c.command(name, args...)
	}
	return exec.Command(name, args...)
}

// NewController создает новый экземпляр контроллера
func NewController() *Controller {
	return &Controller{}
}

// OpenFolder открывает переданный путь в системном файловом менеджере.
// Поддерживает Windows, macOS и Linux.
func (c *Controller) OpenFolder(folderPath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// В Windows пробелы в путях требуют правильной экранизации,
		// explorer обрабатывает это автоматически, если передать путь аргументом
		cmd = exec.Command("explorer", folderPath)
	case "darwin": // macOS
		cmd = exec.Command("open", folderPath)
	default: // Linux
		cmd = exec.Command("xdg-open", folderPath)
	}

	// Используем Start(), а не Run().
	// Run() заблокирует выполнение Go-программы до тех пор, пока папка не будет закрыта.
	// Start() просто запускает процесс в фоне и идет дальше.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error opening folder: %w", err)
	}
	return nil
}

// LaunchAssociated opens an untracked file through the OS association. Direct
// executables are launched by the durable monitor in monitor.go.
func (c *Controller) LaunchAssociated(path, folder string) error {
	if c.associate != nil {
		return c.associate(path, folder)
	}
	return openAssociated(path, folder)
}

// LaunchUntracked uses the same dispatch without a session monitor or library writes.
func (c *Controller) LaunchUntracked(path, folder string) error {
	if !CanTrack(path) {
		return c.LaunchAssociated(path, folder)
	}
	cmd := c.cmd(path)
	cmd.Dir = folder
	detachUtility(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// isDirectExecutable — можно ли запустить файл напрямую (а не через ассоциацию ОС).
// Windows: только .exe/.com. Linux/macOS: файл с битом исполнения, кроме документов
// и бандлов .app (у распакованных из архивов html/swf бит исполнения бывает случайно).
func isDirectExecutable(path, ext string) bool {
	if runtime.GOOS == "windows" {
		return ext == ".exe" || ext == ".com"
	}
	switch ext {
	case ".pdf", ".txt", ".app", ".html", ".htm", ".swf", ".jar", ".qsp", ".rags", ".love", ".url", ".exe":
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// FindExecutables сканирует папку игры и возвращает список путей к .exe файлам.
func (c *Controller) FindExecutables(folderPath string) ([]string, error) {
	var exes []string

	// Проходимся по файлам в папке.
	// filepath.WalkDir работает быстрее, чем старый filepath.Walk, так как не вызывает os.Stat для каждого файла
	err := filepath.WalkDir(folderPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Игнорируем ошибки доступа к системным или скрытым папкам
		}

		if !d.IsDir() {
			name := strings.ToLower(d.Name())
			if strings.HasSuffix(name, ".exe") {
				// Отфильтровываем мусорные экзешники (деинсталляторы и репортеры крашей)
				if !strings.Contains(name, "unins") && !strings.Contains(name, "crash") {
					exes = append(exes, path)
				}
			}
		}

		// Оптимизация: мы не хотим глубоко лезть в папки с ассетами.
		// Ограничимся корнем и первым уровнем вложенности.
		relPath, _ := filepath.Rel(folderPath, path)
		depth := len(strings.Split(relPath, string(os.PathSeparator)))
		if d.IsDir() && depth > 2 {
			return filepath.SkipDir // Пропускаем глубокие директории
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error searching for .exe files: %w", err)
	}

	return exes, nil
}
