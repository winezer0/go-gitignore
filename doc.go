// Package ignore 提供纯 Go 的 Git 风格忽略规则编译、动态目录规则栈和只读仓库
// 匹配器。该包仅依赖标准库，不使用 CGO。
//
// # 选择匹配器
//
// CompileLines 和 CompileFile 将单个规则来源编译为 RuleSet，适合内存规则、指定
// ignore 文件或不需要目录层级的调用方。RuleSet.MatchesPathHow 接收相对于规则
// 根的路径，并返回最后生效规则的来源、行号、原文和否定状态。
//
// IgnoreStack 适合与目录遍历器配合。进入目录时调用 Push，离开时调用 Pop；Clone
// 可为子目录创建独立状态。IgnoreStack 是可变对象，调用方不得在多个 goroutine
// 之间并发修改或读写同一个实例。
//
// RepositoryMatcher 在构造时扫描规则文件并形成固定快照，之后可由多个 goroutine
// 并发查询任意根目录内路径。规则文件发生变化后必须重新创建匹配器。
//
//	config := ignore.DefaultRepositoryConfig()
//	config.IgnoreFileNames = []string{
//		".gitignore",
//		".ignore",
//		".rgignore",
//		".toolignore",
//	}
//	matcher, err := ignore.NewRepositoryMatcherWithConfig(".", config)
//	if err != nil {
//		return err
//	}
//	ignored, detail, err := matcher.MatchesPathHow("build/output.bin", false)
//
// # 规则与优先级
//
// 支持注释、转义、否定、目录规则、根锚定、尾随空格、*、?、Git **、字符类和
// POSIX 字符类。父目录一旦被排除，其内容不能由更深目录中的否定规则重新包含。
//
// RepositoryConfig.IgnoreFileNames 按数组顺序从低到高确定同一目录内的来源优先级，
// 默认顺序为 .gitignore、.ignore、.rgignore。AdditionalIgnoreFiles 按配置顺序
// 加载并具有高于逐目录规则的优先级。默认配置还会读取 .git/info/exclude 和全局
// Git ignore；不需要这些来源时可在 RepositoryConfig 中关闭。
//
// # 路径与错误
//
// RepositoryMatcher 接受根目录内的绝对或相对路径。路径越过规则根目录时返回错误，
// 边界检查采用词法路径语义，不要求目标路径已经存在。isDir 必须准确表示目标是否
// 为目录，否则尾随斜杠目录规则可能得到不同结果。
//
// 规则文件读取、无效 glob、路径转换和仓库初始化错误均返回调用方。启用全局 Git
// ignore 时会尝试执行 git config，命令不可用时回退到标准用户配置路径。当前不解析
// worktree .git 指针文件指向的实际 gitdir。
package goignore
