package cmd

import (
	"fmt"
	"sync" // 用于并发处理 '*' 情况

	"synclink/internal/config"
	"synclink/internal/link" // 导入链接处理逻辑
	"synclink/internal/util"

	"github.com/spf13/cobra"
)

// relinkCmd represents the relink command
var relinkCmd = &cobra.Command{
	Use:   "relink <link_name>",
	Short: "检查并重新链接已管理的符号链接或快捷方式",
	Long: `检查指定名称（或使用 '*' 检查所有）的链接是否存在并且是预期的类型（符号链接或快捷方式）。
如果链接丢失或不正确，则尝试根据存储的配置信息重新创建它。`,
	Args: cobra.ExactArgs(1), // 需要正好一个参数: link_name 或 '*'
	RunE: runRelink,
}

func init() {
	rootCmd.AddCommand(relinkCmd)
}

func runRelink(cmd *cobra.Command, args []string) error {
	linkName := args[0]

	cfg, err := config.GetConfig() // 加载或获取配置
	if err != nil {
		return err
	}

	if linkName == "*" {
		return relinkAllLinks(cfg)
	} else {
		return relinkSingleLink(cfg, linkName)
	}
}

// relinkSingleLink 处理单个链接的重新链接逻辑
func relinkSingleLink(cfg *config.Config, name string) error {
	_, exists := cfg.GetLink(name)
	if !exists {
		return fmt.Errorf("链接 '%s' 未被 synclink 管理。", name)
	}

	fmt.Printf("正在检查链接 '%s'...\n", name)
	err := link.RelinkLinkOrShortcut(name) // 内部会再次加载配置获取详细信息
	if err != nil {
		return fmt.Errorf("尝试重新链接 '%s' 时出错: %v", name, err)
	} else {
		fmt.Printf("链接 '%s' 检查完毕，状态正常或已成功重新链接。\n", name)
		return nil
	}
}

// relinkAllLinks 处理重新链接所有已管理链接的逻辑
func relinkAllLinks(cfg *config.Config) error {
	links := cfg.GetLinks() // 获取所有链接的映射副本

	if len(links) == 0 {
		fmt.Println("没有已管理的链接可供重新链接。")
		return nil
	}

	fmt.Printf("开始检查并重新链接所有 %d 个已管理的链接...\n", len(links))

	var wg sync.WaitGroup
	var successCount int
	var failureCount int
	var lastErr error
	var mu sync.Mutex // 用于保护计数器

	for name := range links {
		wg.Add(1)
		go func(linkNameToRelink string) { // 使用 go routine 并发处理 (可选)
			defer wg.Done()

			fmt.Printf("[+] 正在处理链接 '%s'...\n", linkNameToRelink)
			lastErr = link.RelinkLinkOrShortcut(linkNameToRelink)

			mu.Lock() // 锁定以更新计数器
			if lastErr != nil {
				util.ErrorPrint("[-] 重新链接 '%s' 失败: %v\n", linkNameToRelink, lastErr)
				failureCount++
			} else {
				fmt.Printf("[-] 链接 '%s' 重新链接成功或状态正常。\n", linkNameToRelink)
				successCount++
			}
			mu.Unlock() // 解锁

		}(name) // 传递 name 的副本给 goroutine
	}

	wg.Wait() // 等待所有 goroutine 完成

	fmt.Println("\n重新链接操作完成。")
	fmt.Printf("总计：%d 个链接\n", len(links))
	fmt.Printf("成功：%d 个\n", successCount)
	fmt.Printf("失败：%d 个\n", failureCount)

	return nil
}
