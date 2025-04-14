// cmd/link.go
package cmd

import (
	"fmt"

	"synclink/internal/config"
	"synclink/internal/link"
	"synclink/internal/util"

	"github.com/spf13/cobra"
)

var (
	linkName       string
	syncPath       string
	createShortcut bool
)

// linkCmd represents the link command
var linkCmd = &cobra.Command{
	Use:   "link <target_path>",
	Short: "移动目标路径到同步目录并创建符号链接，或创建快捷方式",
	Long: `将指定的 'target_path' (文件或文件夹) 移动到配置的同步目录下，
并在原始位置创建一个指向新位置的符号链接。这有助于将配置文件等纳入同步范围。

或者，使用 --shortcut 标志，可以在开始菜单中为 'target_path' 创建一个快捷方式。

示例:
  synclink link C:\Users\CurrentUser\AppData\Roaming\MyApp\config.json
  synclink link D:\PortableApps\my-app -n MyPortableApp
  synclink link "C:\Program Files\MyTool\tool.exe" --shortcut
  synclink link "D:\Games\GameLauncher.exe" --shortcut -n MyGameLauncher`,
	Args: cobra.ExactArgs(1), // 需要且仅需要一个参数: target_path
	RunE: runLinkCommand,
}

func init() {
	rootCmd.AddCommand(linkCmd)

	// 定义命令行标志
	linkCmd.Flags().StringVarP(&linkName, "name", "n", "", "指定链接的名称 (默认为目标路径的基本名称)")
	linkCmd.Flags().StringVarP(&syncPath, "sync-path", "s", "", "指定同步目录的路径 (默认为配置中的 DefaultSyncPath)")
	linkCmd.Flags().BoolVar(&createShortcut, "shortcut", false, "创建开始菜单快捷方式而不是符号链接")

}

func runLinkCommand(cmd *cobra.Command, args []string) error {
	targetPath := args[0]

	// 1. 加载配置
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	// 2. 验证和处理输入参数
	targetPath, err = util.GetAbsPath(targetPath)
	if err != nil {
		return err
	}

	// 检查 targetPath 是否存在
	exists, err := util.PathExists(targetPath)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("目标路径 '%s' 不存在。", targetPath)
	}

	// 确定 linkName
	if linkName == "" {
		linkName = util.GetDefaultLinkName(targetPath)
	}

	// 检查 linkName 是否已存在于配置中
	if _, exists := cfg.GetLink(linkName); exists {
		return fmt.Errorf("链接名称 '%s' 已存在，请使用不同的名称，或先使用 'synclink unlink %s' 删除现有链接。", linkName, linkName)
	}
	// 确定 syncPathBase
	syncPathBase := syncPath
	if syncPathBase == "" {
		settings := cfg.GetSettings()
		if settings.DefaultSyncPath == "" {
			return fmt.Errorf("未指定同步路径 (-s)，且配置中未设置默认同步路径，请使用 'synclink config set default_sync_path <路径>' 设置。")
		}
		syncPathBase = settings.DefaultSyncPath
	}

	// 3. 执行核心逻辑
	return link.CreateLinkOrShortcut(targetPath, linkName, syncPathBase, createShortcut)
}
