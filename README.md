# go-gitignore

`go-gitignore` 是纯 Go 的 Git ignore 规则库，可用于目录遍历、代码索引器和其他需要
仓库级文件过滤的工具。模块不依赖 CGO 或第三方 Go 包。

## 功能

- `.gitignore` 风格的注释、转义、否定和尾随空格。
- `*`、`?`、Git `**`、字符类及 POSIX 字符类。
- 根锚定、目录规则和父目录排除语义。
- 可配置的逐目录规则文件及规则优先级。
- `.git/info/exclude`、全局 Git ignore 和额外规则文件。
- 匹配来源、行号、原始规则及否定状态诊断。
- 动态 `IgnoreStack` 和并发只读 `RepositoryMatcher`。
- Windows、Linux 和 macOS 路径处理。

## 安装

```bash
go get github.com/winezer0/gitignore
```

## 仓库匹配器

```go
package main

import (
	"fmt"
	"log"

	ignore "github.com/winezer0/gitignore"
)

func main() {
	config := ignore.DefaultRepositoryConfig()
	config.IgnoreFileNames = []string{
		".gitignore",
		".ignore",
		".rgignore",
		".toolignore",
	}

	matcher, err := ignore.NewRepositoryMatcherWithConfig(".", config)
	if err != nil {
		log.Fatal(err)
	}
	ignored, detail, err := matcher.MatchesPathHow("build/output.bin", false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ignored=%t detail=%+v\n", ignored, detail)
}
```

`IgnoreFileNames` 按数组顺序从低到高确定优先级，默认值为：

```go
[]string{".gitignore", ".ignore", ".rgignore"}
```

`RepositoryMatcher` 在创建时读取规则并形成固定快照，之后可供多个 goroutine
并发查询。规则文件变化后需要重新创建匹配器。

## 动态规则栈

目录遍历器可使用 `IgnoreStack`，进入目录时调用 `Push`，离开时调用 `Pop`：

```go
stack := ignore.NewIgnoreStack(false, false, 0)
if err := stack.LoadBaseRules(root); err != nil {
	return err
}
if err := stack.Push(root); err != nil {
	return err
}
defer stack.Pop()
```

## 行为边界

- 匹配路径必须位于配置的规则根目录内。
- 仓库匹配器进行词法路径边界检查，不要求目标路径已经存在。
- `.git` 为 worktree 指针文件时暂不解析其实际 gitdir。
- 启用全局 Git ignore 时会尝试执行 `git config`，失败后检查标准用户配置路径。

## 许可证

MIT，派生与参考信息见 `THIRD_PARTY_NOTICES.md`。
