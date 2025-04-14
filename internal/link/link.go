// internal/link/link.go
package link

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"synclink/internal/config"
	"synclink/internal/util"
)

// CreateSymbolicLink 处理创建符号链接的逻辑：
// 1. 将 targetPath 移动到 syncDir 下。
// 2. 在 targetPath 的原始位置创建指向新位置的符号链接。
// 3. 将链接信息添加到配置中。
// targetPath: 用户指定的需要被链接的原始文件或文件夹路径。
// linkName: 用户为这个链接指定的名称 (用于配置和 syncDir 中的命名)。
// syncDir: 同步目录的基础路径 (例如 config.Settings.DefaultSyncPath)。
func CreateSymbolicLink(targetPath, linkName, syncDir string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	// --- 验证输入 ---
	absTargetPath, err := util.GetAbsPath(targetPath)
	if err != nil {
		return err
	}

	exists, err := util.PathExists(absTargetPath)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("目标路径 '%s' 不存在", absTargetPath)
	}

	if _, exists := cfg.GetLink(linkName); exists {
		return fmt.Errorf("链接名称 '%s' 已存在", linkName)
	}

	isDir, _ := util.IsDir(absTargetPath) // 忽略错误，因为 PathExists 已确认存在
	isFile, _ := util.IsFile(absTargetPath)

	// --- 计算同步路径 ---
	var syncedPath string
	if isFile {
		// 文件存储在 syncDir/files/linkName 下
		filesDir := filepath.Join(syncDir, "files")
		if err := util.EnsureDirExists(filesDir); err != nil {
			return err
		}
		syncedPath = filepath.Join(filesDir, linkName) // 使用 linkName 作为文件名
	} else if isDir {
		// 文件夹存储在 syncDir/linkName 下
		syncedPath = filepath.Join(syncDir, linkName)
		// 检查目标 syncPath 是否已存在内容，避免意外覆盖
		syncPathExists, _ := util.PathExists(syncedPath)
		if syncPathExists {
			// 可以选择返回错误，或者提供覆盖选项（当前返回错误）
			return fmt.Errorf("同步目标路径 '%s' 已存在", syncedPath)
		}
		// 确保 syncPath 的父目录存在 (虽然 Join 通常不需要，但 EnsureDirExists 更安全)
		if err := util.EnsureDirExists(filepath.Dir(syncedPath)); err != nil {
			return fmt.Errorf("无法创建同步目录的父目录 '%s': %w", filepath.Dir(syncedPath), err)
		}
	} else {
		// 既不是文件也不是目录（可能是特殊文件、损坏的链接等），不支持
		return fmt.Errorf("目标路径 '%s' 不是常规文件或目录，不支持链接", absTargetPath)
	}

	// --- 执行移动和链接 ---
	fmt.Printf("正在移动 '%s' 到 '%s'...\n", absTargetPath, syncedPath)
	// 注意：MoveFileOrDir 的基础实现可能没有跨磁盘进度条
	if err := util.MoveFileOrDir(absTargetPath, syncedPath); err != nil {
		// 尝试清理：如果 syncedPath 被部分创建，可能需要删除
		// os.RemoveAll(syncedPath) // 可选的清理步骤
		return fmt.Errorf("移动 '%s' 到 '%s' 失败: %w", absTargetPath, syncedPath, err)
	}

	fmt.Printf("正在创建符号链接 '%s' -> '%s'...\n", absTargetPath, syncedPath)
	if err := os.Symlink(syncedPath, absTargetPath); err != nil {
		// 尝试回滚移动操作
		if errMoveBack := util.MoveFileOrDir(syncedPath, absTargetPath); errMoveBack != nil {
			util.WarningPrint("回滚移动操作失败！ '%s' 可能需要手动恢复到 '%s'。%v\n",
				syncedPath, absTargetPath, errMoveBack)
		}
		return fmt.Errorf("创建符号链接 '%s' 失败: %w", absTargetPath, err)
	}

	// --- 更新配置 ---
	linkInfo := config.LinkInfo{
		Shortcut:     false, // 明确标记为非快捷方式
		OriginalPath: absTargetPath,
		SyncedPath:   syncedPath,
		CreatedAt:    time.Now(),
	}

	if err := cfg.AddLink(linkName, linkInfo); err != nil {
		// 严重问题：操作已完成但配置未保存
		// 尝试清理（删除链接，移回文件），但这很复杂且风险高
		// 更好的做法是通知用户手动检查
		util.WarningPrint("物理链接已创建，但更新配置文件失败！请手动检查 config.json。错误: %v", err)
		return fmt.Errorf("链接已创建 '%s'，但保存配置失败: %w", linkName, err)
	}

	fmt.Printf("成功创建并记录符号链接 '%s'.\n", linkName)
	return nil
}

// RemoveSymbolicLink 处理移除符号链接的逻辑：
// 1. 删除 originalPath 处的符号链接。
// 2. 将 syncedPath 的内容移回 originalPath。
// 3. 从配置中移除链接信息。
// linkName: 要移除的链接的名称。
func RemoveSymbolicLink(linkName string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	linkInfo, exists := cfg.GetLink(linkName)
	if !exists {
		return fmt.Errorf("链接 '%s' 未在配置中找到", linkName)
	}

	if linkInfo.Shortcut {
		return fmt.Errorf("链接 '%s' 是一个快捷方式，请使用 unlink shortcut 命令（或确保逻辑分离）", linkName)
	}

	if linkInfo.OriginalPath == "" || linkInfo.SyncedPath == "" {
		// 数据不完整，可能配置已损坏
		return fmt.Errorf("链接 '%s' 的配置信息不完整 (original_path 或 synced_path 为空)", linkName)
	}

	// --- 验证状态 ---
	originalExists, _ := util.PathExists(linkInfo.OriginalPath)
	isSymlink := false
	if originalExists {
		isSymlink, _ = util.IsSymlink(linkInfo.OriginalPath)
		if !isSymlink {
			// OriginalPath 存在但不是符号链接，警告用户，继续尝试删除配置
			util.WarningPrint("原始路径 '%s' 存在但不是预期的符号链接。将仅尝试移除配置和移动同步数据（如果存在）。\n", linkInfo.OriginalPath)
		} else {
			// 验证符号链接目标是否正确（可选但推荐）
			currentTarget, err := os.Readlink(linkInfo.OriginalPath)
			if err == nil && currentTarget != linkInfo.SyncedPath {
				util.WarningPrint("符号链接 '%s' 的目标 ('%s') 与配置中的 ('%s') 不匹配。仍将继续移除。\n", linkInfo.OriginalPath, currentTarget, linkInfo.SyncedPath)
			}
		}
	}

	syncedExists, _ := util.PathExists(linkInfo.SyncedPath)
	if !syncedExists {
		// 同步数据丢失，这很严重
		util.WarningPrint("同步路径 '%s' 不存在！无法将数据移回。", linkInfo.SyncedPath)
		// 决定是否继续删除链接和配置
		// return fmt.Errorf("同步路径 '%s' 不存在，无法恢复原始文件/文件夹", linkInfo.SyncedPath) // 更严格的选择
	}

	// --- 执行移除和移动 ---
	if isSymlink { // 仅当原始位置确实是符号链接时才删除
		if err := os.Remove(linkInfo.OriginalPath); err != nil {
			// 如果删除失败，可能不应该继续移动，因为原始位置可能被占用
			return fmt.Errorf("删除符号链接 '%s' 失败: %w", linkInfo.OriginalPath, err)
		}
	} else if originalExists {
		fmt.Printf("跳过删除 '%s'，因为它不是符号链接。\n", linkInfo.OriginalPath)
		// 如果原始路径存在但不是链接，移动操作可能会失败或覆盖用户文件！
		// 增加检查，如果原始路径存在且非空，则中止移动。
		isEmpty := true
		if info, err := os.Stat(linkInfo.OriginalPath); err == nil {
			if info.IsDir() {
				C, err := os.ReadDir(linkInfo.OriginalPath)
				isEmpty = (err == nil && len(C) == 0)
			} else {
				isEmpty = (info.Size() == 0)
			}
		}
		if !isEmpty {
			errMsg := fmt.Sprintf("原始路径 '%s' 存在且非空，并且不是预期的符号链接。为防止数据丢失，取消将 '%s' 移回的操作。请手动处理。", linkInfo.OriginalPath, linkInfo.SyncedPath)
			// 仍然尝试删除配置记录
			_, removeErr := cfg.RemoveLink(linkName)
			if removeErr != nil {
				errMsg += fmt.Sprintf(" (移除配置记录也失败: %v)", removeErr)
			} else {
				errMsg += " (配置记录已移除)"
			}
			return errors.New(errMsg)
		}
		fmt.Printf("原始路径 '%s' 存在但非符号链接，且为空，将尝试移动内容...\n", linkInfo.OriginalPath)
	}

	if syncedExists {
		if err := util.MoveFileOrDir(linkInfo.SyncedPath, linkInfo.OriginalPath); err != nil {
			// 移动失败，这也很麻烦
			// 此时符号链接（如果存在且被删除）已删除，但数据仍在同步位置
			return fmt.Errorf("无法将 '%s' 移回 '%s': %w。请手动恢复。", linkInfo.SyncedPath, linkInfo.OriginalPath, err)
		}
	} else {
		util.WarningPrint("跳过移回操作，因为同步路径 '%s' 不存在。", linkInfo.SyncedPath)
	}

	// --- 更新配置 ---
	removed, err := cfg.RemoveLink(linkName)
	if err != nil {
		return fmt.Errorf("物理文件/链接已处理，但从配置中移除 '%s' 失败: %w", linkName, err)
	}
	if !removed {
		// 这理论上不应该发生，因为我们开始时检查了 exists
		util.WarningPrint("尝试移除链接 '%s'，但配置中似乎已不存在。", linkName)
	}
	return nil
}

// RelinkSymbolicLink 检查符号链接是否存在且正确，如果不存在则尝试重新创建。
// linkName: 要检查和可能重新链接的链接名称。
func RelinkSymbolicLink(linkName string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	linkInfo, exists := cfg.GetLink(linkName)
	if !exists {
		return fmt.Errorf("链接 '%s' 未在配置中找到", linkName)
	}

	if linkInfo.Shortcut {
		return fmt.Errorf("链接 '%s' 是一个快捷方式，请使用 relink shortcut 命令（或确保逻辑分离）", linkName)
	}

	if linkInfo.OriginalPath == "" || linkInfo.SyncedPath == "" {
		return fmt.Errorf("链接 '%s' 的配置信息不完整", linkName)
	}

	// --- 检查当前状态 ---
	originalExists, _ := util.PathExists(linkInfo.OriginalPath)
	needsRelink := false

	if !originalExists {
		fmt.Printf("符号链接 '%s' 不存在，需要重新创建。\n", linkInfo.OriginalPath)
		needsRelink = true
	} else {
		isSymlink, _ := util.IsSymlink(linkInfo.OriginalPath)
		if !isSymlink {
			// 路径存在但不是符号链接，这是一个冲突！不能自动解决。
			return fmt.Errorf("路径 '%s' 存在但不是符号链接，无法重新链接。请手动解决冲突", linkInfo.OriginalPath)
		} else {
			// 是符号链接，检查它是否指向正确的位置
			currentTarget, err := os.Readlink(linkInfo.OriginalPath)
			if err != nil {
				// 读取链接目标失败，可能链接损坏
				util.WarningPrint("无法读取符号链接 '%s' 的目标: %v。将尝试重新创建。", linkInfo.OriginalPath, err)
				// 尝试删除损坏的链接
				if errRem := os.Remove(linkInfo.OriginalPath); errRem != nil {
					return fmt.Errorf("无法移除损坏的符号链接 '%s'，重新链接失败: %w", linkInfo.OriginalPath, errRem)
				}
				needsRelink = true
			} else if currentTarget != linkInfo.SyncedPath {
				fmt.Printf("符号链接 '%s' 指向 '%s' 而不是预期的 '%s'。将尝试修正。\n", linkInfo.OriginalPath, currentTarget, linkInfo.SyncedPath)
				// 删除错误的链接
				if errRem := os.Remove(linkInfo.OriginalPath); errRem != nil {
					return fmt.Errorf("无法移除指向错误的符号链接 '%s'，重新链接失败: %w", linkInfo.OriginalPath, errRem)
				}
				needsRelink = true
			} else {
				// 链接存在且正确
				// fmt.Printf("符号链接 '%s' -> '%s' 已存在且正确。\n", linkInfo.OriginalPath, linkInfo.SyncedPath)
				return nil // 无需操作
			}
		}
	}

	// --- 如果需要，执行重新链接 ---
	if needsRelink {
		// 在重新创建链接之前，必须确保同步目标仍然存在
		syncedExists, _ := util.PathExists(linkInfo.SyncedPath)
		if !syncedExists {
			return fmt.Errorf("同步路径 '%s' 不存在，无法重新创建链接 '%s'", linkInfo.SyncedPath, linkInfo.OriginalPath)
		}

		fmt.Printf("正在重新创建符号链接 '%s' -> '%s'...\n", linkInfo.OriginalPath, linkInfo.SyncedPath)
		if err := os.Symlink(linkInfo.SyncedPath, linkInfo.OriginalPath); err != nil {
			return fmt.Errorf("重新创建符号链接 '%s' 失败: %w", linkInfo.OriginalPath, err)
		}
		fmt.Println("符号链接重新创建成功.")
	}

	return nil
}

// --- 快捷方式处理函数 (定义接口，实现在 shortcut_*.go) ---

// CreateShortcutDelegate 是创建快捷方式的实际实现。
// 需要在特定平台的 _windows.go 文件中实现。
// targetPath: 快捷方式指向的目标文件/文件夹。
// linkName: 在配置和快捷方式文件中使用的名称。
// startMenuPathBase: 通常是 Start Menu Programs 目录。
var CreateShortcutDelegate func(targetPath, linkName, startMenuPathBase string) (shortcutFilePath string, err error)

// RemoveShortcutDelegate 是移除快捷方式的实际实现。
// linkName: 要移除的链接名称 (用于查找配置和快捷方式文件)。
// startMenuPathBase: 通常是 Start Menu Programs 目录。
var RemoveShortcutDelegate func(linkName, startMenuPathBase string, linkInfo config.LinkInfo) error

// RelinkShortcutDelegate 是重新链接快捷方式的实际实现。
// linkName: 要重新链接的链接名称。
// startMenuPathBase: 通常是 Start Menu Programs 目录。
// linkInfo: 从配置加载的链接信息。
var RelinkShortcutDelegate func(linkName, startMenuPathBase string, linkInfo config.LinkInfo) error

// GetStartMenuProgramsPathDelegate 获取特定平台的开始菜单程序路径。
var GetStartMenuProgramsPathDelegate func() (string, error)

// CreateLinkOrShortcut 根据参数决定是创建符号链接还是快捷方式。
func CreateLinkOrShortcut(targetPath, linkName, syncPathBase string, isShortcut bool) error {
	if isShortcut {
		if CreateShortcutDelegate == nil || GetStartMenuProgramsPathDelegate == nil {
			return errors.New("创建快捷方式的功能在此系统上不受支持或未正确初始化")
		}
		startMenuPath, err := GetStartMenuProgramsPathDelegate()
		if err != nil {
			return fmt.Errorf("无法获取开始菜单路径: %w", err)
		}

		absTargetPath, err := util.GetAbsPath(targetPath)
		if err != nil {
			return fmt.Errorf("获取 '%s' 的绝对路径失败: %w", targetPath, err)
		}

		// 检查目标是否存在
		exists, err := util.PathExists(absTargetPath)
		if err != nil {
			return fmt.Errorf("检查路径 '%s' 时出错: %w", absTargetPath, err)
		}
		if !exists {
			return fmt.Errorf("快捷方式的目标路径 '%s' 不存在", absTargetPath)
		}

		// 调用特定平台的实现来创建快捷方式物理文件
		shortcutFilePath, err := CreateShortcutDelegate(absTargetPath, linkName, startMenuPath)
		if err != nil {
			return err // CreateShortcutDelegate 应返回具体的错误信息
		}

		// 更新配置
		cfg, err := config.GetConfig()
		if err != nil {
			// 物理快捷方式已创建，但配置失败，需要警告
			util.WarningPrint("快捷方式文件 '%s' 已创建，但加载/保存配置失败: %v。请手动检查配置。\n", shortcutFilePath, err)
			return fmt.Errorf("快捷方式 '%s' 已创建，但处理配置失败: %w", linkName, err)
		}

		linkInfo := config.LinkInfo{
			Shortcut:     true,
			OriginalPath: absTargetPath,    // 快捷方式的目标
			SyncedPath:   shortcutFilePath, // 对于快捷方式，我们将 SyncedPath 用于存储 .lnk 文件的路径
			CreatedAt:    time.Now(),
		}

		if err := cfg.AddLink(linkName, linkInfo); err != nil {
			// 尝试清理已创建的快捷方式文件
			util.WarningPrint("快捷方式文件 '%s' 已创建，但保存配置失败: %v。正在尝试移除快捷方式文件...\n", shortcutFilePath, err)
			if remErr := os.Remove(shortcutFilePath); remErr != nil {
				util.WarningPrint("移除快捷方式文件 '%s' 失败: %v\n", shortcutFilePath, remErr)
			}
			return fmt.Errorf("快捷方式 '%s' 已创建，但保存配置失败: %w", linkName, err)
		}
		fmt.Printf("成功创建并记录快捷方式 '%s' (位于 '%s')。\n", linkName, shortcutFilePath)
	} else {
		// 创建符号链接
		if err := CreateSymbolicLink(targetPath, linkName, syncPathBase); err != nil {
			return err
		}
	}
	return nil
}

// RemoveLinkOrShortcut 根据配置信息决定是移除符号链接还是快捷方式。
func RemoveLinkOrShortcut(linkName string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	linkInfo, exists := cfg.GetLink(linkName)
	if !exists {
		return fmt.Errorf("链接 '%s' 未在配置中找到", linkName)
	}

	var removalErr error
	if linkInfo.Shortcut {
		// 快捷方式移除逻辑
		if RemoveShortcutDelegate == nil || GetStartMenuProgramsPathDelegate == nil {
			removalErr = errors.New("移除快捷方式的功能在此系统上不受支持或未正确初始化")
		} else {
			startMenuPath, pathErr := GetStartMenuProgramsPathDelegate()
			if pathErr != nil {
				// 如果无法获取路径，则无法确定快捷方式位置，但仍尝试删除配置
				util.WarningPrint("无法获取开始菜单路径以移除快捷方式: %w。将仅尝试移除配置记录。", pathErr)
			} else {
				// 调用特定平台的实现来删除快捷方式物理文件
				removalErr = RemoveShortcutDelegate(linkName, startMenuPath, linkInfo)
				if removalErr != nil {
					// 保留错误，但下面会尝试删除配置
					util.WarningPrint("移除快捷方式文件时出错: %v。仍将尝试移除配置记录。", removalErr)
				}
			}
		}
	} else {
		// 符号链接移除逻辑（包括将文件移回）
		removalErr = RemoveSymbolicLink(linkName) // RemoveSymbolicLink 内部已处理配置移除
		if removalErr != nil {
			return fmt.Errorf("移除符号链接 '%s' 失败: %w", linkName, removalErr)
		}
		return nil
	}

	// --- 更新配置 (仅当是快捷方式时，因为 RemoveSymbolicLink 已处理) ---
	if linkInfo.Shortcut { // 只有快捷方式需要在这里显式删除配置
		removed, configErr := cfg.RemoveLink(linkName)
		if configErr != nil {
			// 物理移除可能已成功（或失败），但配置移除失败
			if removalErr != nil {
				// 两个操作都失败了
				return fmt.Errorf("移除快捷方式文件失败 (%v) 并且移除配置记录也失败: %w", removalErr, configErr)
			}
			// 物理移除成功，但配置移除失败
			return fmt.Errorf("快捷方式文件已处理，但从配置中移除 '%s' 失败: %w", linkName, configErr)
		}
		if !removed && removalErr == nil { // 物理移除成功，但配置中未找到？
			util.WarningPrint("尝试移除链接 '%s'，但配置中似乎已不存在（尽管物理移除已尝试/成功）。", linkName)
		}
		// 如果 removalErr 不为 nil，表示物理移除失败，但配置移除成功，返回物理移除的错误
		if removalErr != nil {
			return fmt.Errorf("移除快捷方式文件失败: %w (配置记录已移除)", removalErr)
		}
	}
	return nil // 如果一切顺利到达这里
}

// RelinkLinkOrShortcut 根据配置信息决定是重新链接符号链接还是快捷方式。
func RelinkLinkOrShortcut(linkName string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	linkInfo, exists := cfg.GetLink(linkName)
	if !exists {
		return fmt.Errorf("链接 '%s' 未在配置中找到", linkName)
	}

	if linkInfo.Shortcut {
		// 快捷方式重新链接逻辑
		if RelinkShortcutDelegate == nil || GetStartMenuProgramsPathDelegate == nil {
			return errors.New("重新链接快捷方式的功能在此系统上不受支持或未正确初始化")
		}
		startMenuPath, err := GetStartMenuProgramsPathDelegate()
		if err != nil {
			return fmt.Errorf("无法获取开始菜单路径以重新链接快捷方式: %w", err)
		}
		// 调用特定平台的实现来检查和重新创建快捷方式
		err = RelinkShortcutDelegate(linkName, startMenuPath, linkInfo)
		if err != nil {
			return fmt.Errorf("重新链接快捷方式 '%s' 失败: %w", linkName, err)
		}
	} else {
		// 符号链接重新链接逻辑
		err = RelinkSymbolicLink(linkName)
		if err != nil {
			return fmt.Errorf("重新链接符号链接 '%s' 失败: %w", linkName, err)
		}
	}

	return nil
}
