package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/artifact"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func cmdArtifact(args []string) int {
	if len(args) == 1 && args[0] == "list" {
		files, err := artifact.List()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_ = json.NewEncoder(os.Stdout).Encode(files)
		return 0
	}
	if len(args) == 2 && args[0] == "remove" {
		if len(args[1]) != 32 || strings.Trim(args[1], "0123456789abcdef") != "" {
			return 2
		}
		if err := artifact.Remove(args[1]); err != nil {
			diag.For("artifact").Act("act.artifact", diag.Caller(), "", diag.Failed, "document removal failed", "id", args[1])
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		diag.For("artifact").Act("act.artifact", diag.Caller(), "", diag.OK, "document copy removed", "id", args[1])
		return 0
	}
	if len(args) == 0 || args[0] != "add" {
		i18n.Sae("Usage: gtmux artifact add FILE --session-id ID [--home-id ID] [--json]\nAlso: gtmux artifact list | gtmux artifact remove ID\nPublishes one private, immutable document for your paired devices; guests cannot read it. The original file is unchanged.", "用法：gtmux artifact add 文件 --session-id 会话ID [--home-id 目录身份] [--json]\n其他操作：gtmux artifact list | gtmux artifact remove ID\n发布一个私有文件快照，供本人已配对设备查看；访客不能读取。原文件不变。")
		if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
			return 0
		}
		return 2
	}
	var source, session, home string
	structured := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--session-id", "--home-id":
			if i+1 >= len(args) {
				return 2
			}
			flag := args[i]
			i++
			if flag == "--session-id" {
				session = args[i]
			} else {
				home = args[i]
			}
		case "--json":
			structured = true
		default:
			if strings.HasPrefix(args[i], "-") || source != "" {
				return 2
			}
			source = args[i]
		}
	}
	if source == "" || session == "" {
		i18n.Sae("A file and its conversation ID are required.", "请指定文件和所属会话 ID。")
		return 2
	}
	file, err := artifact.Add(source, session, home)
	if err != nil {
		diag.For("artifact").Act("act.artifact", diag.Caller(), session, diag.Failed, "document publication failed")
		i18n.Sae("Could not publish document: "+err.Error(), "无法提供文件预览："+err.Error())
		return 1
	}
	diag.For("artifact").Act("act.artifact", diag.Caller(), session, diag.OK, "document published", "id", file.ID, "bytes", file.Bytes)
	if structured {
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			artifact.File
			Reference string `json:"reference"`
		}{file, file.Reference()})
	} else {
		fmt.Printf("[%s](%s)\n", strings.NewReplacer("[", "", "]", "").Replace(file.Name), file.Reference())
	}
	return 0
}
