//go:build windows

// internal/link/shortcut_windows.go
package link

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"synclink/internal/config"
	"synclink/internal/util"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// init 函数在 Windows 平台初始化时，将具体的快捷方式处理函数赋值给 link.go 中定义的委托变量。
func init() {
	CreateShortcutDelegate = createShortcutWindows
	RemoveShortcutDelegate = removeShortcutWindows
	RelinkShortcutDelegate = relinkShortcutWindows
	GetStartMenuProgramsPathDelegate = getStartMenuProgramsPathWindows
}

// createShortcutWindows 在 Windows 上创建 .lnk 快捷方式。
// targetPath: 快捷方式指向的目标文件或文件夹的绝对路径。
// linkName: 用于配置文件和快捷方式文件名的名称 (不含 .lnk 后缀)。
// startMenuPathBase: 开始菜单程序文件夹的根路径。
func createShortcutWindows(targetPath, linkName, startMenuPathBase string) (shortcutFilePath string, err error) {
	shortcutFilePath = filepath.Join(startMenuPathBase, linkName+".lnk")
	targetPath, err = filepath.Abs(targetPath) // 确保目标路径是绝对路径
	if err != nil {
		return "", fmt.Errorf("无法获取目标 '%s' 的绝对路径: %w", targetPath, err)
	}

	// 确保快捷方式所在的目录存在
	if err := util.EnsureDirExists(startMenuPathBase); err != nil {
		return "", err
	}

	// 初始化 COM 库
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_DISABLE_OLE1DDE); err != nil {
		// S_FALSE 表示已经初始化
		if !strings.Contains(err.Error(), "S_FALSE") {
			return "", fmt.Errorf("初始化 COM 失败: %w", err)
		}
	}
	defer ole.CoUninitialize() // 确保在函数退出时释放 COM

	// 创建 WScript.Shell 对象
	oleShellObject, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return "", fmt.Errorf("创建 WScript.Shell 对象失败: %w", err)
	}
	defer oleShellObject.Release() // 确保释放对象

	wshell, err := oleShellObject.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return "", fmt.Errorf("获取 WScript.Shell 的 IDispatch 接口失败: %w", err)
	}
	defer wshell.Release()

	// 调用 CreateShortcut 方法创建快捷方式对象
	cs, err := oleutil.CallMethod(wshell, "CreateShortcut", shortcutFilePath)
	if err != nil {
		return "", fmt.Errorf("调用 CreateShortcut 方法失败: %w", err)
	}

	shortcut := cs.ToIDispatch() // 获取快捷方式对象的 IDispatch 接口
	if shortcut == nil {
		return "", fmt.Errorf("无法获取快捷方式对象的 IDispatch 接口")
	}
	defer shortcut.Release()

	// 设置快捷方式属性
	// 1. 设置目标路径
	if _, err = oleutil.PutProperty(shortcut, "TargetPath", targetPath); err != nil {
		return "", fmt.Errorf("设置快捷方式 TargetPath ('%s') 失败: %w", targetPath, err)
	}

	// 2. 设置工作目录 (通常设置为目标文件所在的目录)
	workingDir := filepath.Dir(targetPath)
	if _, err = oleutil.PutProperty(shortcut, "WorkingDirectory", workingDir); err != nil {
		// 工作目录设置失败通常不是致命错误，可以记录日志或忽略
		util.WarningPrint("设置快捷方式 WorkingDirectory ('%s') 失败: %v", workingDir, err)
	}

	// 3. 设置描述 (可选)
	description := fmt.Sprintf("由 synclink 管理的快捷方式，指向: %s", targetPath)
	if _, err = oleutil.PutProperty(shortcut, "Description", description); err != nil {
		util.WarningPrint("设置快捷方式 Description 失败: %v", err)
	}

	// 4. 设置图标位置 (可选, 尝试使用目标本身的图标)
	//    如果目标是 exe，通常可以这样设置；如果是文件夹或其他类型，可能无效
	isExe, _ := util.IsFile(targetPath) // 简单判断，不严谨
	isExe = strings.HasSuffix(strings.ToLower(targetPath), ".exe") && isExe
	if isExe {
		iconLocation := fmt.Sprintf("%s,0", targetPath) // 使用目标文件的第一个图标
		if _, err = oleutil.PutProperty(shortcut, "IconLocation", iconLocation); err != nil {
			util.WarningPrint("设置快捷方式 IconLocation ('%s') 失败: %v", iconLocation, err)
		}
	}

	// 保存快捷方式文件
	if _, err = shortcut.CallMethod("Save"); err != nil {
		return "", fmt.Errorf("保存快捷方式文件 '%s' 失败: %w", shortcutFilePath, err)
	}

	return shortcutFilePath, nil
}

// removeShortcutWindows 在 Windows 上删除 .lnk 快捷方式。
// linkName: 要移除的链接的名称 (用于查找 .lnk 文件)。
// startMenuPathBase: 开始菜单程序文件夹的根路径。
// linkInfo: (在此函数中未使用，但签名需要匹配)
func removeShortcutWindows(linkName, startMenuPathBase string, linkInfo config.LinkInfo) error {
	shortcutFilePath := filepath.Join(startMenuPathBase, linkName+".lnk")
	exists, err := util.PathExists(shortcutFilePath)
	if err != nil {
		return err
	}
	if !exists {
		return nil // 文件不存在，视为成功删除 (幂等性)
	}
	// 尝试删除文件
	if err := os.Remove(shortcutFilePath); err != nil {
		return fmt.Errorf("删除快捷方式 '%s' 失败: %w", shortcutFilePath, err)
	}
	return nil
}

// relinkShortcutWindows 检查 Windows 上的快捷方式是否存在，如果不存在则重新创建。
// 注意：此实现仅检查快捷方式文件是否存在，不验证其内部指向是否正确。
// 更健壮的实现会读取 .lnk 文件并验证 TargetPath，但会更复杂。
// 简单的重新链接策略是：删除旧的（如果存在），然后创建新的。
func relinkShortcutWindows(linkName, startMenuPathBase string, linkInfo config.LinkInfo) error {
	shortcutFilePath := filepath.Join(startMenuPathBase, linkName+".lnk")

	// 1. 先尝试删除旧的快捷方式（忽略错误，因为它可能不存在）
	err := removeShortcutWindows(linkName, startMenuPathBase, linkInfo)
	if err != nil {
		// 记录删除失败的警告，但继续尝试创建
		util.WarningPrint("在重新链接前删除旧快捷方式 '%s' 时遇到问题: %v", shortcutFilePath, err)
	}

	// 2. 使用配置信息重新创建快捷方式
	_, err = createShortcutWindows(linkInfo.OriginalPath, linkName, startMenuPathBase)
	if err != nil {
		return fmt.Errorf("重新创建快捷方式 '%s' 失败: %w", linkName, err)
	}

	return nil
}

// getStartMenuProgramsPathWindows 获取 Windows 当前用户的 "开始菜单 -> 程序" 文件夹路径。
func getStartMenuProgramsPathWindows() (string, error) {
	// 通常，开始菜单程序路径位于 %APPDATA%\Microsoft\Windows\Start Menu\Programs
	appData := os.Getenv("APPDATA")
	if appData == "" {
		// 尝试使用 UserProfile 作为后备 （不太标准，但有时 APPDATA 未设置）
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			return "", fmt.Errorf("无法获取 APPDATA 或 USERPROFILE 环境变量，无法确定开始菜单路径")
		}
		// 在 UserProfile 下的路径可能不同，这里用标准 APPDATA 结构尝试
		appData = filepath.Join(userProfile, "AppData", "Roaming")
		util.WarningPrint("APPDATA 环境变量未设置，尝试使用基于 USERPROFILE 的路径:", appData)
	}

	startMenuPath := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")

	// 检查路径是否存在，如果不存在可能需要报错或返回空（取决于策略）
	// 这里我们先返回构造的路径，让调用者处理不存在的情况
	// if _, err := os.Stat(startMenuPath); os.IsNotExist(err) {
	// 	return "", fmt.Errorf("计算出的开始菜单程序路径 '%s' 不存在", startMenuPath)
	// }

	return startMenuPath, nil
}
