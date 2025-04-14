// cmd/unlink.go
package cmd

import (
	"fmt"

	"synclink/internal/config" // 确保导入路径相对于你的项目模块根目录正确
	"synclink/internal/link"   // 确保导入路径正确
	"synclink/internal/util"

	"github.com/spf13/cobra"
)

// unlinkCmd represents the unlink command
var unlinkCmd = &cobra.Command{
	Use:   "unlink <link_name>",
	Short: "移除一个已管理的链接或快捷方式",
	Long: `根据名称移除一个由 synclink 管理的符号链接或快捷方式。

对于符号链接：
1. synclink 会删除在原始位置创建的符号链接。
2. synclink 会将之前移动到同步目录的数据移回其原始位置。
3. synclink 会从配置文件中移除该链接的记录。

对于快捷方式：
1. synclink 会删除在启动菜单中创建的快捷方式文件。
2. synclink 会从配置文件中移除该快捷方式的记录。

特别地，如果 link_name 是 '*'，则会尝试移除所有当前管理的链接和快捷方式。`,
	Args: cobra.ExactArgs(1), // 必须提供一个参数：链接名称或 '*'
	RunE: func(cmd *cobra.Command, args []string) error {
		linkName := args[0]

		cfg, err := config.GetConfig()
		if err != nil {
			return err
		}

		// 处理通配符 '*'
		if linkName == "*" {
			allLinks := cfg.GetLinks() // 获取所有链接信息的副本
			if len(allLinks) == 0 {
				fmt.Println("没有找到任何已管理的链接或快捷方式。")
				return nil
			}

			successCount := 0
			failCount := 0

			for name := range allLinks {
				err := link.RemoveLinkOrShortcut(name) // 核心移除逻辑
				if err != nil {
					util.ErrorPrint("[-] 移除 '%s' 失败: %v\n", name, err)
				} else {
					fmt.Printf("[-] 已移除 '%s'\n", name)
					successCount++
				}
			}
			fmt.Println("\n移除链接操作完成。")
			fmt.Printf("总计：%d 个链接\n", len(allLinks))
			fmt.Printf("成功：%d 个\n", successCount)
			fmt.Printf("失败：%d 个\n", failCount)

			return nil
		}

		// 处理单个链接名称
		_, exists := cfg.GetLink(linkName)
		if !exists {
			return fmt.Errorf("未在配置中找到名为 '%s' 的链接或快捷方式", linkName)
		}

		err = link.RemoveLinkOrShortcut(linkName)
		if err != nil {
			return err
		}

		fmt.Printf("已成功移除 '%s'\n", linkName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(unlinkCmd) // 将 unlink 命令添加到根命令
}
