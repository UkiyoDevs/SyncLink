// synclink/cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"synclink/internal/config" // 引入内部配置包
	"synclink/internal/util"
)

// rootCmd 代表没有调用子命令时的基础命令
var rootCmd = &cobra.Command{
	Use:   "synclink",
	Short: "一个通过符号链接/快捷方式同步文件和配置的工具",
	Long: `Synclink 旨在帮助管理应用程序配置或其他文件/文件夹的同步。

它可以将指定的目标移动到统一的同步目录中，
并在原始位置创建符号链接（用于实际文件同步）或
在开始菜单创建快捷方式（用于快速访问），
从而简化跨设备或备份场景下的文件管理。

灵感来源于 Scoop ，但提供了更灵活的配置和管理方式。`,
	// PersistentPreRunE 会在任何子命令执行 *之前* 运行。
	// 这是加载配置的理想位置，确保所有子命令都能访问到配置。
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 尝试加载应用程序配置
		_, err := config.LoadConfig()
		if err != nil {
			// 如果加载配置失败，则向用户报告错误并阻止命令继续执行
			// 使用 fmt.Errorf 包装原始错误以提供更多上下文
			return fmt.Errorf("加载配置文件失败: %w", err)
		}
		// 配置加载成功，可以继续执行子命令
		return nil
	},
	// 如果用户只输入 "synclink" 而没有子命令，默认行为是显示帮助信息。
	// Cobra 默认就是这样，所以不需要显式设置 Run。
}

// Execute 将所有子命令添加到根命令中，并适当设置标志。
// 这是 main.main() 调用的主要函数。
func Execute() {
	rootCmd.SilenceErrors = true
	// 执行 rootCmd
	// rootCmd.Execute() 会解析命令行参数，找到匹配的子命令并执行
	err := rootCmd.Execute()
	if err != nil {
		util.ErrorPrint(err.Error())
		os.Exit(1)
	}
}
