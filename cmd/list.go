// cmd/list.go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"synclink/internal/config" // 确认你的 module path

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有 synclink 管理的链接",
	Long:  `列出当前配置文件中记录的所有符号链接和快捷方式的详细信息。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. 加载配置
		cfg, err := config.GetConfig()
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		// 2. 获取链接信息
		linksMap := cfg.GetLinks() // GetLinks 返回一个副本，不需要额外加锁

		if len(linksMap) == 0 {
			fmt.Println("当前没有管理的链接。")
			return nil
		}

		// 初始化 tablewriter
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"链接名称", "类型", "原始路径", "同步路径", "创建时间"})
		// table.SetAutoWrapText(true)
		// table.SetAutoFormatHeaders(true)
		// table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		// table.SetAlignment(tablewriter.ALIGN_LEFT)
		// table.SetCenterSeparator("")
		// table.SetColumnSeparator("")
		// table.SetRowSeparator("")
		// table.SetHeaderLine(true)
		// table.SetBorder(true)
		// table.SetTablePadding("  ") // 列间距
		// table.SetNoWhiteSpace(true)

		// 填充数据
		for name := range linksMap {
			info := linksMap[name]
			var linkType, displayPath string

			displayPath = info.SyncedPath
			if info.Shortcut {
				linkType = "快捷方式"
				if strings.Contains(info.SyncedPath, "Start Menu") {
					displayPath = "开始菜单"
				}
			} else {
				linkType = "符号链接"
			}

			createdAtStr := info.CreatedAt.Format("2006-01-02 15:04:05")
			table.Append([]string{
				name,
				linkType,
				info.OriginalPath,
				displayPath,
				createdAtStr,
			})
		}

		// 渲染表格
		fmt.Println("\n当前管理的链接列表:")
		table.Render()
		fmt.Printf("\n总共管理 %d 个链接。\n", len(linksMap))

		return nil // 表示成功
	},
}

func init() {
	// 将 listCmd 添加到 rootCmd
	// rootCmd 应该在 cmd/root.go 中定义
	rootCmd.AddCommand(listCmd)

	// 这里可以为 list 命令添加本地 flags (如果需要的话)
	// listCmd.Flags().BoolP("details", "d", false, "显示更详细的信息")
}
