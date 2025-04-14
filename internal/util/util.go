package util

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall" // 用于检查跨设备链接错误

	"github.com/fatih/color"
	"golang.org/x/sys/windows"
)

// ConfigFileName 是配置文件的默认名称
const ConfigFileName = "config.json"

// 定义颜色配置
var (
	warningColor = color.New(color.FgYellow) // 黄色
	errorColor   = color.New(color.FgRed)    // 红色
)

// 定义直接绑定到 os.Stderr 的打印函数变量
var (
	WarningPrint = func(format string, a ...interface{}) {
		warningColor.Fprintf(os.Stderr, format, a...) // 固定输出到 stderr
	}

	ErrorPrint = func(format string, a ...interface{}) {
		errorColor.Fprintf(os.Stderr, format, a...)
	}
)

// GetExecutableDir 返回当前运行的可执行文件所在的目录。
func GetExecutableDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	return filepath.Dir(exePath), nil
}

// GetConfigPath 返回配置文件的绝对路径 (默认在可执行文件同目录下)。
func GetConfigPath() (string, error) {
	exeDir, err := GetExecutableDir()
	if err != nil {
		return "", err // 错误已经被包装
	}
	return filepath.Join(exeDir, ConfigFileName), nil
}

// PathExists 检查给定路径的文件或目录是否存在。
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil // 路径存在
	}
	if os.IsNotExist(err) {
		return false, nil // 路径不存在
	}
	// 其他错误 (例如权限问题)
	return false, fmt.Errorf("检查路径 '%s' 时出错: %w", path, err)
}

// IsDir 检查给定路径是否是一个目录。
// 如果路径不存在或不是目录，则返回 false。
func IsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // 不存在不是目录
		}
		return false, fmt.Errorf("无法获取路径 '%s' 的信息: %w", path, err)
	}
	return info.IsDir(), nil
}

// IsFile 检查给定路径是否是一个普通文件。
// 如果路径不存在或不是普通文件，则返回 false。
func IsFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // 不存在不是文件
		}
		return false, fmt.Errorf("无法获取路径 '%s' 的信息: %w", path, err)
	}
	// 确保它不是目录、符号链接、设备文件等
	return info.Mode().IsRegular(), nil
}

// IsSymlink 检查给定路径是否是一个符号链接。
func IsSymlink(path string) (bool, error) {
	info, err := os.Lstat(path) // 使用 Lstat 而不是 Stat 来获取链接本身的信息
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // 不存在不是符号链接
		}
		return false, fmt.Errorf("无法获取路径 '%s' 的 Lstat 信息: %w", path, err)
	}
	// 检查文件模式是否包含符号链接标志
	return info.Mode()&os.ModeSymlink != 0, nil
}

// EnsureDirExists 确保指定的目录存在，如果不存在则创建它（包括所有父目录）。
func EnsureDirExists(dirPath string) error {
	// 使用 0755 权限创建目录，这是一个常见的默认值
	// os.MkdirAll 在目录已存在时不会返回错误
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("创建目录 '%s' 失败: %w", dirPath, err)
	}
	return nil
}

// MoveFileOrDir 移动文件或目录。
// 它会尝试使用 os.Rename，如果失败（特别是跨设备链接错误），
// 则会回退到复制然后删除源文件/目录的方式。
// 注意：跨磁盘移动的进度条需要更复杂的实现（例如使用 io.Copy 和回调），
// 这个基础版本暂不包含进度条。
func MoveFileOrDir(src, dst string) error {
	// 1. 尝试直接重命名 (在同一文件系统下速度最快)
	err := os.Rename(src, dst)
	if err == nil {
		return nil // 移动成功
	}

	// 在不同系统上错误类型可能不同
	isCrossDevice := false
	// Linux/macOS: syscall.EXDEV
	// Windows: 检查错误消息或特定的错误码 (可能不直接暴露为 EXDEV)
	// 这里我们使用一个更通用的检查方式，虽然可能不够完美
	// 在 Go 1.8+ 中，syscall.EXDEV 对于 Windows 上的 ERROR_NOT_SAME_DEVICE 也会被转换
	if errors.Is(err, syscall.EXDEV) || errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) { // 检查底层错误
		isCrossDevice = true
	}

	// 如果错误不是预期的跨设备错误，则直接返回错误
	if !isCrossDevice {
		return fmt.Errorf("移动 '%s' 到 '%s' 失败: %w", src, dst, err)
	}

	// 3. 如果是跨设备错误，则执行复制和删除操作
	// (此处可以添加进度条逻辑)

	isDir, err := IsDir(src)
	if err != nil {
		return fmt.Errorf("无法确定源路径 '%s' 类型: %w", src, err)
	}

	if isDir {
		// 复制目录
		if err := CopyDir(src, dst); err != nil {
			return fmt.Errorf("复制目录 '%s' 到 '%s' 失败: %w", src, dst, err)
		}
	} else {
		// 复制文件
		if err := CopyFile(src, dst); err != nil {
			return fmt.Errorf("复制文件 '%s' 到 '%s' 失败: %w", src, dst, err)
		}
	}

	// 4. 复制成功后，删除源文件/目录
	if err := os.RemoveAll(src); err != nil {
		// 重要：如果删除失败，目标位置可能已经有了副本，这是一个不一致的状态
		// 实际应用中可能需要更复杂的事务处理或回滚逻辑
		return fmt.Errorf("复制成功后删除源 '%s' 失败: %w", src, err)
	}

	return nil // 移动成功
}

// CopyFile 复制单个文件从 src 到 dst。
// 它会尝试保留原始文件的权限。如果目标文件已存在，它将被覆盖。
// 如果目标目录不存在，会尝试创建它。
func CopyFile(src, dst string) error {
	// 确保目标目录存在
	dstDir := filepath.Dir(dst)
	if err := EnsureDirExists(dstDir); err != nil {
		return fmt.Errorf("无法创建目标目录 '%s': %w", dstDir, err)
	}

	// 打开源文件
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("无法打开源文件 '%s': %w", src, err)
	}
	defer sourceFile.Close() // 确保文件句柄被关闭

	// 获取源文件信息 (用于权限)
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("无法获取源文件 '%s' 的信息: %w", src, err)
	}

	// 创建目标文件 (使用 O_TRUNC 来覆盖已存在的文件)
	destFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("无法创建目标文件 '%s': %w", dst, err)
	}
	defer destFile.Close() // 确保文件句柄被关闭

	// 使用 io.Copy 进行高效复制
	// (如果需要进度条，需要使用 io.CopyBuffer 和一个自定义的 Reader/Writer)
	bytesCopied, err := io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("复制文件内容从 '%s' 到 '%s' 失败: %w", src, dst, err)
	}

	// 验证复制的字节数是否与源文件大小一致
	if bytesCopied != sourceInfo.Size() {
		// 尝试清理未完全写入的目标文件
		_ = destFile.Close() // 先关闭
		_ = os.Remove(dst)   // 再删除
		return fmt.Errorf("复制字节数不匹配 (%d vs %d) for '%s' -> '%s'", bytesCopied, sourceInfo.Size(), src, dst)
	}

	// 确保所有数据都写入磁盘 (Sync)
	err = destFile.Sync()
	if err != nil {
		// Sync 错误通常不致命，但最好记录下来
		WarningPrint("同步目标文件 '%s' 到磁盘时出错: %v", dst, err)
		// 不返回错误，因为内容已经复制了
	}

	// 尝试设置目标文件的权限 (与源文件相同)
	// 注意：在某些系统或文件系统上，这可能不完全成功或不被支持
	err = os.Chmod(dst, sourceInfo.Mode())
	if err != nil {
		WarningPrint("设置目标文件 '%s' 权限失败: %v", dst, err)
		// 不返回错误，因为主要复制操作已完成
	}

	return nil
}

// CopyDir 递归地复制整个目录从 src 到 dst。
// 如果目标目录 dst 不存在，它将被创建。
// 如果目标目录或其中的子项已存在，它们的行为取决于 CopyFile（文件会被覆盖）。
func CopyDir(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	// 获取源目录信息
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("无法获取源目录 '%s' 信息: %w", src, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源路径 '%s' 不是一个目录", src)
	}

	// 创建目标根目录 (如果不存在)
	// 使用源目录的权限模式
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("无法创建目标目录 '%s': %w", dst, err)
	}

	// 使用 WalkDir 遍历源目录
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		// 1. 处理 WalkDir 本身遇到的错误 (例如权限问题)
		if err != nil {
			return fmt.Errorf("遍历 '%s' 时出错: %w", path, err)
		}

		// 2. 计算对应的目标路径
		// relPath 是当前项相对于源目录的路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			// 这理论上不应该发生，因为 path 总是 src 的子路径
			return fmt.Errorf("无法计算相对路径 '%s' from '%s': %w", path, src, err)
		}
		targetPath := filepath.Join(dst, relPath)

		// 3. 跳过根目录本身 (因为它已经被创建或已存在)
		if path == src {
			return nil
		}

		// 4. 根据类型处理
		if d.IsDir() {
			// 如果是目录，则在目标位置创建它
			// 获取原始目录的权限
			info, dirErr := d.Info()
			if dirErr != nil {
				return fmt.Errorf("无法获取目录 '%s' 的信息: %w", path, dirErr)
			}
			if err := os.MkdirAll(targetPath, info.Mode()); err != nil {
				return fmt.Errorf("无法在目标位置创建目录 '%s': %w", targetPath, err)
			}
		} else if d.Type().IsRegular() { // 确保是普通文件 (跳过符号链接等)
			// 如果是文件，则复制它
			if err := CopyFile(path, targetPath); err != nil {
				// 错误已经被包装在 CopyFile 内部了
				return err // 直接返回错误，停止 Walk
			}
		} else {
			// 可以选择性地处理符号链接、设备文件等，或直接跳过
			fmt.Printf("跳过非普通文件/目录: %s (类型: %s)\n", path, d.Type())
		}

		return nil // 继续遍历
	})

	if err != nil {
		// 如果 WalkDir 返回错误 (来自我们的回调函数或 WalkDir 本身)
		return fmt.Errorf("复制目录 '%s' 到 '%s' 过程中失败: %w", src, dst, err)
	}

	return nil
}

// GetAbsPath 获取绝对路径，如果已经是绝对路径则直接返回，否则相对于 PWD 解析。
func GetAbsPath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	absPath, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("获取路径 '%s' 的绝对路径失败: %w", p, err)
	}
	return absPath, nil
}

func GetDefaultLinkName(p string) string {
	name := filepath.Base(p)
	lowerName := strings.ToLower(name)
	if strings.HasSuffix(lowerName, ".exe") {
		name = name[:len(name)-4]
	}
	return name
}
