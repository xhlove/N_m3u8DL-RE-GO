package main

import (
	"fmt"
	"os"

	"N_m3u8DL-RE-GO/internal/command"
	"N_m3u8DL-RE-GO/internal/util"
)

const VERSION_INFO = "N-M3U8DL-RE-GO (Beta version) 20250615"

func main() {
	// 初始化控制台
	util.InitConsole(false, false)

	// 初始化日志文件
	if err := util.Logger.InitLogFile(); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志文件失败: %v\n", err)
	}

	// 输出版本信息
	util.Logger.InfoMarkUp("[deepskyblue1]%s[/]", VERSION_INFO)

	// 执行命令
	if err := command.Execute(); err != nil {
		util.Logger.ErrorMarkUp("程序执行失败: %s", err.Error())
		os.Exit(1)
	}
}
