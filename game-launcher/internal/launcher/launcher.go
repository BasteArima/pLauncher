package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Controller отвечает за взаимодействие с файловой системой ОС и процессами
type Controller struct{}

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

// LaunchGame запускает исполняемый файл игры.
// Если задан onExit, в фоне (через cmd.Wait — без поллинга, нулевая нагрузка)
// дожидается завершения процесса и сообщает время сессии в минутах.
func (c *Controller) LaunchGame(exePath, folderPath string, onExit func(minutes int)) error {
	ext := strings.ToLower(filepath.Ext(exePath))

	// Прямой запуск нативных исполняемых: так ОС отдаёт хэндл процесса,
	// и cmd.Wait позволяет считать время игры без поллинга.
	if isDirectExecutable(exePath, ext) {
		cmd := exec.Command(exePath)
		// Рабочей директорией должна быть папка игры, иначе Unity/RenPy не найдут ассеты.
		cmd.Dir = folderPath
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("couldn't launch %s: %w", exePath, err)
		}
		if onExit != nil {
			go func() {
				start := time.Now()
				cmd.Wait() // блокируется на хэндле процесса, ОС будит при выходе
				onExit(int(time.Since(start).Minutes()))
			}()
		}
		return nil
	}

	// Остальные форматы (html/swf/jar/qsp/rags/lnk/bat…, на macOS — .app) открываем
	// ассоциированным приложением. Процесс отсоединяется, поэтому время игры для них
	// не учитывается (last_launched_at всё равно проставляется на стороне app.go).
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", "start", "", "/D", folderPath, exePath)
	case "darwin":
		cmd = exec.Command("open", exePath)
	default:
		cmd = exec.Command("xdg-open", exePath)
	}
	cmd.Dir = folderPath
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't launch %s: %w", exePath, err)
	}
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
	case ".app", ".html", ".htm", ".swf", ".jar", ".qsp", ".rags", ".love", ".url", ".exe":
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
