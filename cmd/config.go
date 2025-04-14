package cmd

import (
	"errors"
	"fmt"
	"strings"

	"synclink/internal/config" // 导入配置包

	"github.com/spf13/cobra"
)

// configCmd 代表 config 命令
var configCmd = &cobra.Command{
	Use:   "config <get|set> <属性> [新值]",
	Short: "获取或设置 synclink 的配置项",
	Long: `管理 synclink 的配置。

你可以使用 'get' 来查看一个配置项的值，或者使用 'set' 来修改它。

支持的属性:
  default_sync_path: 默认的同步目录路径

示例:
  synclink config get default_sync_path
  synclink config set default_sync_path D:\MySyncFolder`,
	Args: func(cmd *cobra.Command, args []string) error {
		// 至少需要两个参数 (操作 和 属性)
		if len(args) < 2 {
			return errors.New("缺少参数。需要指定操作 (get/set) 和属性名称")
		}
		// 将操作转换为小写以方便比较
		action := strings.ToLower(args[0])
		if action != "get" && action != "set" {
			return fmt.Errorf("无效的操作 '%s'。只支持 'get' 或 'set'", args[0])
		}
		// 'set' 操作需要第三个参数 (新值)
		if action == "set" && len(args) < 3 {
			return errors.New("使用 'set' 操作时必须提供新值")
		}
		// 'get' 操作只需要两个参数
		if action == "get" && len(args) > 2 {
			return errors.New("使用 'get' 操作时不需要提供额外的值")
		}
		// 'set' 操作只需要三个参数
		if action == "set" && len(args) > 3 {
			return errors.New("使用 'set' 操作时提供了过多的参数")
		}

		// 检查属性名称是否有效 (目前只支持 default_sync_path)
		// 将属性名也转为小写以方便比较
		attributeName := strings.ToLower(args[1])
		if attributeName != "default_sync_path" {
			// 可以在这里扩展支持更多属性
			return fmt.Errorf("不支持的配置属性: '%s'", args[1])
		}

		return nil // 参数校验通过
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// 加载或获取配置实例
		cfg, err := config.GetConfig()
		if err != nil {
			return fmt.Errorf("加载配置文件失败: %w", err)
		}

		action := strings.ToLower(args[0])
		attributeName := strings.ToLower(args[1]) // 保证与校验时一致

		switch action {
		case "get":
			switch attributeName {
			case "default_sync_path":
				// GetSettings() 返回的是结构体副本或实际值，可以直接访问
				settings := cfg.GetSettings()
				fmt.Printf("default_sync_path: %s\n", settings.DefaultSyncPath)
			default:
				// Arg 函数理论上应该已经阻止了这种情况
				return fmt.Errorf("未知属性 '%s'", attributeName)
			}
		case "set":
			newValue := args[2]
			switch attributeName {
			case "default_sync_path":
				// 调用 SetDefaultSyncPath，它内部会处理保存逻辑
				err := cfg.SetDefaultSyncPath(newValue)
				if err != nil {
					return fmt.Errorf("设置 default_sync_path 失败: %w", err)
				}
				fmt.Printf("成功将 default_sync_path 设置为: %s\n", newValue)
			default:
				// Arg 函数理论上应该已经阻止了这种情况
				return fmt.Errorf("内部错误：遇到未知的属性 '%s'", attributeName)
			}
		default:
			// Arg 函数理论上应该已经阻止了这种情况
			return fmt.Errorf("内部错误：遇到未知的操作 '%s'", action)
		}

		return nil // 操作成功
	},
}

// init 函数在程序启动时自动被调用
func init() {
	// 将 configCmd 添加到 rootCmd
	// 假设 rootCmd 在 cmd/root.go 中定义并导出
	rootCmd.AddCommand(configCmd)
}

// 注意:
// 1. 确保在 cmd/root.go (或其他地方) 定义了 rootCmd。
// 2. 确保 internal/config/config.go 中的 GetConfig 和 SetDefaultSyncPath 函数按预期工作，
//    特别是 SetDefaultSyncPath 应该包含调用 SaveConfig 的逻辑。
// 3. 错误处理返回 error，Cobra 会负责打印错误信息。
// 4. 用户输出使用 fmt.Printf 直接打印到标准输出。
