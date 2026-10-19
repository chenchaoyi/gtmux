package app

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/dshfeed"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func cmdMessageTranscript(args []string) int {
	after, review, limit := int64(-1), int64(-1), 100
	home := ""
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--messages" || arg == "--json" {
			continue
		}
		if i+1 >= len(args) {
			return 2
		}
		i++
		if arg == "--home-id" {
			home = args[i]
			continue
		}
		n, err := strconv.ParseInt(args[i], 10, 64)
		if err != nil {
			return 2
		}
		switch arg {
		case "--after-seq":
			after = n
		case "--review-through":
			review = n
		case "--limit":
			if n < 1 || n > 200 {
				return 2
			}
			limit = int(n)
		default:
			return 2
		}
	}
	if after < -1 || review < -1 {
		return 2
	}
	ref, err := sessionpolicy.Resolve(args[0], home)
	if err != nil || ref.Agent != "dsh" {
		i18n.Sae("Public message pages require a verified DeepSeek Harness conversation.", "逐条消息视图需要身份已确认的 DeepSeek Harness 会话。")
		return 1
	}
	page, err := dsh.ReadMessages(args[0], after, limit)
	if err == nil && review >= 0 {
		prior, readErr := dshfeed.Reviewed(ref.HomeID, args[0])
		if readErr != nil || after > prior {
			i18n.Sae("Read from the existing review cursor before advancing it.", "请从已有审阅进度开始读取，再推进审阅标记。")
			return 1
		}
		err = dshfeed.Review(ref.HomeID, args[0], review, page)
		if err == nil {
			diag.Did("act.session.review", args[0], diag.OK, "declared public-message review", "home_id", ref.HomeID, "through", review)
		}
	}
	if err == nil {
		page.ReviewedThrough, err = dshfeed.Reviewed(ref.HomeID, args[0])
	}
	if err != nil {
		i18n.Sae("Could not read or review messages: "+err.Error(), "无法读取或标记消息审阅："+err.Error())
		return 1
	}
	b, _ := json.Marshal(page)
	fmt.Println(string(b))
	return 0
}
