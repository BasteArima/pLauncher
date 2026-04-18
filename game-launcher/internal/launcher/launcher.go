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
		return fmt.Errorf("ошибка при открытии папки: %w", err)
	}
	return nil
}

// LaunchGame запускает исполняемый файл игры.
func (c *Controller) LaunchGame(exePath, folderPath string) error {
	cmd := exec.Command(exePath)

	// Крайне важный момент для игр (особенно сделанных на Unity/RenPy):
	// Рабочей директорией (Working Directory) должна быть папка игры,
	// иначе игра не найдет свои ассеты (картинки, звуки) и вылетит с ошибкой.
	cmd.Dir = folderPath

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить %s: %w", exePath, err)
	}

	// TODO в будущем: Если мы захотим трекать время в игре (Time Played),
	// здесь нужно будет запустить горутину с cmd.Wait(), которая засечет
	// время старта и время завершения процесса, а затем обновит БД.

	return nil
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
		return nil, fmt.Errorf("ошибка при поиске .exe файлов: %w", err)
	}

	return exes, nil
}
