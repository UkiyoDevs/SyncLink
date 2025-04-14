// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"synclink/internal/util"
)

// ConfigFileName 是配置文件的名称。
// 导出以便其他包需要时使用，尽管优先使用 GetConfigPath。
const ConfigFileName = "config.json"
const CurrentVersion = "1.0" // 定义配置结构的当前版本

// Settings 保存应用程序的一般设置。
type Settings struct {
	DefaultSyncPath string `json:"default_sync_path"`
}

// LinkInfo 保存单个管理链接的详细信息。
type LinkInfo struct {
	Shortcut     bool      `json:"shortcut"`              // 如果这是快捷方式则为 true，如果是符号链接则为 false
	OriginalPath string    `json:"original_path"`         // 文件/文件夹的原始位置
	SyncedPath   string    `json:"synced_path,omitempty"` // 实际数据存储位置（仅针对符号链接）
	CreatedAt    time.Time `json:"created_at"`            // 链接创建的时间
}

// Config 是应用程序配置的根结构体。
type Config struct {
	Settings Settings            `json:"settings"`
	Links    map[string]LinkInfo `json:"links"`   // 键是链接名称
	Version  string              `json:"version"` // 结构版本
}

var (
	configInstance *Config
	once           sync.Once
	configMutex    sync.RWMutex // 保护对配置和文件的并发访问的互斥锁
	configPath     string       // 缓存配置路径
)

// init 函数一次性确定配置路径。
func init() {
	var err error
	configPath, err = util.GetConfigPath()
	if err != nil {
		// 如果我们甚至无法确定配置路径，那就是一个严重的启动错误。
		// 我们可能想要恐慌或致命日志，具体取决于所需的健壮性。
		// 现在，我们打印错误并继续，LoadConfig 会再次处理它。
		fmt.Fprintf(os.Stderr, "初始化期间确定配置路径时出错: %v\n", err)
		// 将 configPath 设置为空字符串将导致 LoadConfig 后续失败
		configPath = ""
	}
}

// getConfigPath 返回缓存的配置路径。
// 它集中处理路径逻辑和处理初始化错误的情况。
func getConfigPath() (string, error) {
	if configPath == "" {
		// 这可能意味着 init() 函数失败。尝试重新获取或返回错误。
		// 或者，依靠最初的错误打印返回一个通用错误。
		return "", fmt.Errorf("在启动时无法确定配置路径")
	}
	return configPath, nil
}

// LoadConfig 从 JSON 文件加载配置。
// 如果文件不存在，它会创建一个默认配置并保存。
// 它确保加载仅发生一次，使用 sync.Once。
func LoadConfig() (*Config, error) {
	var loadErr error
	once.Do(func() {
		cfgPath, err := getConfigPath()
		if err != nil {
			loadErr = fmt.Errorf("获取配置文件路径失败: %w", err)
			return
		}

		configMutex.Lock() // 锁定以进行初始加载/创建
		defer configMutex.Unlock()

		// 检查文件是否存在
		exists, err := util.PathExists(cfgPath)
		if err != nil {
			loadErr = fmt.Errorf("检查配置文件在 '%s' 是否存在失败: %w", cfgPath, err)
			return
		}

		if !exists {
			fmt.Printf("在 '%s' 找不到配置文件. 正在创建默认配置文件.\n", cfgPath)
			// 创建默认配置
			defaultConfig := &Config{
				Settings: Settings{
					// DefaultSyncPath 在最初可能是空的，需要通过 `config set` 设置
					DefaultSyncPath: "", // 或者可能是相对于可执行文件的 "Sync"？或者用户的主目录？"" 更安全。
				},
				Links:   make(map[string]LinkInfo),
				Version: CurrentVersion,
			}
			// 确保在保存之前目录存在
			if err := util.EnsureDirExists(filepath.Dir(cfgPath)); err != nil {
				loadErr = fmt.Errorf("无法创建配置文件 '%s' 的目录: %w", cfgPath, err)
				return // 如果目录创建失败则不尝试保存
			}
			// 保存默认配置
			if err := saveConfigInternal(cfgPath, defaultConfig); err != nil {
				loadErr = fmt.Errorf("无法保存默认配置文件 '%s': %w", cfgPath, err)
				return // 保存默认配置失败，不分配它
			}
			configInstance = defaultConfig
			fmt.Println("默认配置文件创建成功.")
		} else {
			// 文件存在，加载它
			data, err := os.ReadFile(cfgPath)
			if err != nil {
				loadErr = fmt.Errorf("读取配置文件 '%s' 失败: %w", cfgPath, err)
				return
			}

			// 检查文件是否为空
			if len(data) == 0 {
				// 处理空文件情况 - 或许将其视为缺失？或者记录警告？
				// 让我们尝试解组，可能会出错或导致零值结构体。
				util.WarningPrint("配置文件 '%s' 为空。", cfgPath)
				// 使用默认结构来避免后续的 nil 指针解引用
				configInstance = &Config{
					Settings: Settings{},
					Links:    make(map[string]LinkInfo),
					Version:  "", // 版本将为空，可能需要更新逻辑
				}
				// 尝试解组，也许它只是 `{}`
				if err := json.Unmarshal(data, configInstance); err != nil && string(data) != "{}" && string(data) != "null" {
					// 如果解组失败并且不只是空对象/空值，报告错误
					loadErr = fmt.Errorf("解析空或无效配置文件 '%s' 失败: %w", cfgPath, err)
					configInstance = nil // 确保在错误时实例为 nil
					return
				}
				// 如果解组成功（例如来自 `{}`），确保 Links 映射被初始化
				if configInstance.Links == nil {
					configInstance.Links = make(map[string]LinkInfo)
				}

			} else {
				// 文件非空，尝试解组
				var cfg Config
				// 在解组之前初始化映射，以避免如果 JSON 中缺少 "links" 时的 nil 映射
				cfg.Links = make(map[string]LinkInfo)
				if err := json.Unmarshal(data, &cfg); err != nil {
					loadErr = fmt.Errorf("解析配置文件 '%s' 失败: %w", cfgPath, err)
					return
				}
				// TODO: 如果需要，添加版本检查和潜在的迁移逻辑
				if cfg.Version != CurrentVersion {
					// 处理版本不匹配 - 可能迁移或警告用户
					util.WarningPrint("配置文件版本不匹配（发现 '%s'，需要 '%s'），可能会出现兼容性问题。", cfg.Version, CurrentVersion)
					// 现在，我们继续，但更新内存中的版本
					cfg.Version = CurrentVersion // 或执行实际迁移
				}
				// 即使在 JSON 中为 null 也确保 Links 映射被初始化
				if cfg.Links == nil {
					cfg.Links = make(map[string]LinkInfo)
				}
				configInstance = &cfg
			}
		}
	}) // 结束 once.Do

	// 检查是否在 once.Do 中发生了 loadErr
	if loadErr != nil {
		return nil, loadErr
	}
	// 如果在加载尝试之前出现错误，可能会返回 nil configInstance
	if configInstance == nil && loadErr == nil {
		// 如果 init 失败或其他边缘情况可能会发生
		return nil, fmt.Errorf("在加载尝试后配置实例为 nil")
	}

	return configInstance, nil
}

// GetConfig 返回加载的配置实例。如果尚未加载，则触发 LoadConfig。
func GetConfig() (*Config, error) {
	if configInstance == nil {
		// 如果还未加载，则触发加载
		_, err := LoadConfig()
		if err != nil {
			return nil, err // 返回加载错误
		}
		// 在加载尝试后再次检查
		if configInstance == nil {
			return nil, fmt.Errorf("无法加载或初始化配置")
		}
	}
	// 返回只读副本？对于 CLI 来说，可能直接访问没问题，在写入时受到互斥锁的保护。
	// 目前返回直接指针。调用者在并发修改时应使用互斥锁（尽管 CLI 命令通常不会重叠）。
	return configInstance, nil
}

// SaveConfig 将当前配置状态保存到 JSON 文件中。
// 它获取写入锁以确保安全的并发访问。
func SaveConfig() error {
	configMutex.Lock() // 使用写锁进行保存
	defer configMutex.Unlock()

	if configInstance == nil {
		return fmt.Errorf("配置未加载，无法保存")
	}

	cfgPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("获取保存用配置文件路径失败: %w", err)
	}

	// 在保存之前更新版本（如有必要）
	configInstance.Version = CurrentVersion

	return saveConfigInternal(cfgPath, configInstance)
}

// saveConfigInternal 执行实际的保存逻辑。假设持有锁。
func saveConfigInternal(path string, cfg *Config) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := util.EnsureDirExists(dir); err != nil {
		return fmt.Errorf("确保配置目录 '%s' 存在失败: %w", dir, err)
	}

	// 使用漂亮的打印格式化配置
	data, err := json.MarshalIndent(cfg, "", "  ") // 使用 2 个空格的缩进
	if err != nil {
		return fmt.Errorf("将配置序列化为 JSON 失败: %w", err)
	}

	// 写入文件（os.WriteFile 处理创建/截断和关闭）
	// 使用 0644 权限：所有者读/写，组读，其他人读
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("写入配置文件 '%s' 失败: %w", path, err)
	}

	return nil
}

// === 访问/修改配置的辅助函数 ===

// GetSettings 返回当前设置。
// 建议在确保通过 GetConfig() 加载配置后使用。
func (c *Config) GetSettings() Settings {
	configMutex.RLock() // 读锁
	defer configMutex.RUnlock()
	// 返回副本以防止未使用 Setters 的修改？
	// 对于这个内部包，直接访问可能没问题，或者提供特定的获取器。
	return c.Settings
}

// GetDefaultSyncPath 获取默认同步路径。
func (c *Config) GetDefaultSyncPath() (string, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()
	if c.Settings.DefaultSyncPath == "" {
		return "", fmt.Errorf("配置中未设置 DefaultSyncPath。请使用 'synclink config set default_sync_path <path>'")
	}
	return c.Settings.DefaultSyncPath, nil
}

// SetDefaultSyncPath 设置默认同步路径并保存配置。
func (c *Config) SetDefaultSyncPath(newPath string) error {
	absPath, err := util.GetAbsPath(newPath) // 存储绝对路径
	if err != nil {
		return fmt.Errorf("无效路径 '%s': %w", newPath, err)
	}

	configMutex.Lock() // 完全锁定以便修改
	c.Settings.DefaultSyncPath = absPath
	configMutex.Unlock() // 在保存之前解锁

	return SaveConfig() // 保存隐式地再次处理锁定
}

// GetLinks 返回所有管理链接的映射。
// 返回映射的副本以防止未使用 Add/RemoveLink 的外部修改。
func (c *Config) GetLinks() map[string]LinkInfo {
	configMutex.RLock()
	defer configMutex.RUnlock()
	// 创建副本进行返回
	copiedLinks := make(map[string]LinkInfo, len(c.Links))
	for k, v := range c.Links {
		copiedLinks[k] = v
	}
	return copiedLinks
}

// GetLink 通过其名称检索特定链接的信息。
// 如果找到，则返回 LinkInfo 和 true，否则返回零值和 false。
func (c *Config) GetLink(name string) (LinkInfo, bool) {
	configMutex.RLock()
	defer configMutex.RUnlock()
	info, exists := c.Links[name]
	return info, exists // 返回结构的副本
}

// AddLink 添加或更新链接条目并保存配置。
func (c *Config) AddLink(name string, info LinkInfo) error {
	configMutex.Lock()
	// 确保链接映射已初始化（应通过 LoadConfig 实现）
	if c.Links == nil {
		c.Links = make(map[string]LinkInfo)
	}
	c.Links[name] = info
	configMutex.Unlock()

	return SaveConfig()
}

// RemoveLink 通过其名称移除链接条目并保存配置。
// 如果链接存在并被移除，则返回 true，否则返回 false。
func (c *Config) RemoveLink(name string) (bool, error) {
	var existed bool
	configMutex.Lock()
	if _, ok := c.Links[name]; ok {
		delete(c.Links, name)
		existed = true
	}
	configMutex.Unlock()

	if existed {
		err := SaveConfig()
		if err != nil {
			// 在保存失败时尝试恢复删除的链接到内存中？
			// 为简单起见，我们现在只返回错误。
			return true, fmt.Errorf("链接在内存中已移除但保存配置失败: %w", err)
		}
		return true, nil
	}
	return false, nil // 链接不存在
}
