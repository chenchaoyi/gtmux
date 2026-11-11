package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/screenshot"
)

var screenshotNative screenshot.NativeCall = screenshot.Native

const screenshotHelp = `gtmux screenshot <operation> [--json]
  capabilities                         Query the running Mac host; never opens permissions
  sources --app <bundle-id> | --displays List explicit capture sources (30-second references)
  capture --source-ref REF [--rect x,y,w,h --unit pt] --output FILE.png
  import --input FILE --output FILE.png Convert an existing image without live provenance
  annotate --capture FILE.capture.json --spec MARKS.json --output FILE.png
  edit --document FILE.capture.json    Explicitly open the existing native editor
  target --pane %N [--session-id SID]  Resolve an exact supported live conversation
  send --image FILE --target-ref REF --request-id ID [--message-file FILE]
  status ID                           Read the last durable receipt, not agent completion

Native operations require a compatible running macOS App. No MCP is required.
Capture/annotation never send implicitly. Outputs stay in your chosen task directory.
Send refuses drafts, approvals, changed targets and unknown delivery retries.
Target references currently require a verified managed Codex or DSH native binding;
other agents keep the existing explicit gtmux send --attach interface.
Every operation emits versioned JSON; --timeout defaults to 10s (maximum 10s).
`

func screenshotHelpText() string {
	text := screenshotHelp
	if i18n.Lang() == "zh" {
		text = `gtmux screenshot <operation> [--json]
截图、标注并发送到指定会话。所有操作均输出 JSON，可直接用于脚本。

操作步骤：
  capabilities
    检查 Mac App 的截图和标注能力，不申请录屏权限。
  sources --app <bundle-id> | --displays
    列出指定应用窗口或显示器；来源引用有效期为 30 秒。
  capture --source-ref REF --output FILE.png
    截图并保存；可加 --rect x,y,w,h --unit pt 选择区域。
  import --input FILE --output FILE.png
    导入已有图片，不创建实时截图来源。
  annotate --capture FILE.capture.json --spec MARKS.json --output FILE.png
    按标注文件生成图片；支持箭头、矩形、文字和遮挡。
  edit --document FILE.capture.json
    打开 Mac App 的截图编辑器。
  target --pane %N [--session-id SID]
    核对目标会话，取得发送引用。
  send --image FILE --target-ref REF --request-id ID [--message-file FILE]
    发送图片，可附文字；同一次发送须沿用同一 request-id。
  status ID
    查询已保存的发送回执；回执不代表助手已完成处理。

截图和标注需要兼容的 Mac App 正在运行，无需 MCP。
截图、导入和标注不会自动发送；输出保存在 --output 指定的位置。
有未提交草稿、待确认选项、目标变化或发送结果未知时，会拒绝发送。
精确目标引用目前支持已核实关联的 Codex 和 DSH 会话；
其他助手可使用 gtmux send --attach 显式发送。
--timeout 默认 10 秒，最大 10 秒。
`
	}
	return wrapHelpLines(text, helpWidth())
}

func screenshotOutput(result any, err error) int {
	reply := struct {
		Version int               `json:"schema_version"`
		OK      bool              `json:"ok"`
		Result  any               `json:"result,omitempty"`
		Error   *screenshot.Error `json:"error,omitempty"`
	}{Version: screenshot.Version, OK: err == nil, Result: result}
	if err != nil {
		if receipt, ok := result.(screenshot.Receipt); !ok || receipt.ID == "" {
			reply.Result = nil
		}
		reply.Error = screenshot.AsError(err)
	}
	_ = json.NewEncoder(os.Stdout).Encode(reply)
	if err != nil {
		return 1
	}
	return 0
}

func cmdScreenshot(args []string) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(screenshotHelpText())
		return 0
	}
	op := args[0]
	fs := flag.NewFlagSet("screenshot "+op, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	_ = fs.Bool("json", true, "machine output")
	timeout := fs.Duration("timeout", 10*time.Second, "overall request timeout")
	var app, ref, rect, unit, output, input, document, specPath, pane, session, target, requestID, noteFile string
	var displays bool
	switch op {
	case "capabilities":
	case "sources":
		fs.StringVar(&app, "app", "", "app bundle identifier")
		fs.BoolVar(&displays, "displays", false, "explicitly list displays")
	case "capture":
		fs.StringVar(&ref, "source-ref", "", "source reference")
		fs.StringVar(&rect, "rect", "", "source-local region")
		fs.StringVar(&unit, "unit", "pt", "region units")
		fs.StringVar(&output, "output", "", "output PNG")
	case "import":
		fs.StringVar(&input, "input", "", "input image")
		fs.StringVar(&output, "output", "", "output PNG")
	case "annotate":
		fs.StringVar(&document, "capture", "", "capture document")
		fs.StringVar(&specPath, "spec", "", "annotation JSON")
		fs.StringVar(&output, "output", "", "output PNG")
	case "edit":
		fs.StringVar(&document, "document", "", "editable document")
	case "target":
		fs.StringVar(&pane, "pane", "", "exact pane")
		fs.StringVar(&session, "session-id", "", "expected session")
	case "send":
		fs.StringVar(&input, "image", "", "image")
		fs.StringVar(&target, "target-ref", "", "exact target")
		fs.StringVar(&requestID, "request-id", "", "stable request ID")
		fs.StringVar(&noteFile, "message-file", "", "note file")
	case "status":
	default:
		return screenshotOutput(nil, screenshot.Fail("unknown-operation", "Use gtmux screenshot --help"))
	}
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			fmt.Print(screenshotHelpText())
			return 0
		}
		return screenshotOutput(nil, screenshot.Fail("invalid-options", "Check this operation's flags"))
	}
	if *timeout <= 0 || *timeout > 10*time.Second {
		return screenshotOutput(nil, screenshot.Fail("invalid-options", "Use 0 < timeout <= 10s"))
	}
	if fs.NArg() != 0 && op != "status" {
		return screenshotOutput(nil, screenshot.Fail("invalid-options", "Unexpected positional arguments"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if op == "target" {
		result, err := screenshotResolveTarget(ctx, pane, session)
		return screenshotOutput(result, err)
	}
	if op == "send" {
		result, err := screenshotSend(ctx, input, target, requestID, noteFile)
		return screenshotOutput(result, err)
	}
	if op == "status" {
		if fs.NArg() != 1 {
			return screenshotOutput(nil, screenshot.Fail("request-required", "Provide the request ID"))
		}
		r, err := screenshot.ReadReceipt(fs.Arg(0))
		return screenshotOutput(r, err)
	}
	req := screenshot.Request{Operation: op, App: app, SourceRef: ref}
	if op == "sources" {
		if (app == "") == (!displays) {
			return screenshotOutput(nil, screenshot.Fail("source-required", "Choose --app or --displays"))
		}
		if displays {
			req.SourceKind = "display"
		}
	}
	if op == "capture" {
		if ref == "" || unit != "pt" {
			return screenshotOutput(nil, screenshot.Fail("source-required", "Choose a source reference and pt coordinates"))
		}
		if rect != "" {
			for _, v := range strings.Split(rect, ",") {
				n, err := strconv.ParseFloat(v, 64)
				if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
					return screenshotOutput(nil, screenshot.Fail("invalid-region", "Use x,y,width,height in source-local points"))
				}
				req.Rect = append(req.Rect, n)
			}
			if len(req.Rect) != 4 || req.Rect[0] < 0 || req.Rect[1] < 0 || req.Rect[2] <= 0 || req.Rect[3] <= 0 {
				return screenshotOutput(nil, screenshot.Fail("invalid-region", "Expected a positive source-local region"))
			}
		}
	}
	if op == "import" && input == "" {
		return screenshotOutput(nil, screenshot.Fail("image-required", "Choose an input image"))
	}
	if op == "annotate" || op == "edit" {
		doc, err := screenshot.ReadDocument(document)
		if err != nil {
			return screenshotOutput(nil, err)
		}
		var spec screenshot.Spec
		if op == "edit" && doc.BaseDocument != nil && doc.Annotations != nil {
			spec = *doc.Annotations
			doc = *doc.BaseDocument
		} else if op == "edit" {
			spec = screenshot.Spec{Version: 1, CaptureID: doc.CaptureID, ImageSHA: doc.ImageSHA, Space: "image_px", Marks: []screenshot.Mark{}}
		} else {
			spec, err = screenshot.ReadSpec(specPath)
			if err != nil {
				return screenshotOutput(nil, err)
			}
		}
		if err = spec.Validate(doc); err != nil {
			return screenshotOutput(nil, err)
		}
		if spec.Space == "preview_px" {
			preview, err := screenshot.OpenImage(doc.PreviewPath, doc.PreviewSHA)
			if err != nil {
				return screenshotOutput(nil, err)
			}
			preview.Close()
		}
		req.Document, req.Spec, req.ImageSHA = &doc, &spec, doc.ImageSHA
		input = doc.ImagePath
		if op == "edit" {
			f, err := screenshot.OpenImage(input, doc.ImageSHA)
			if err != nil {
				return screenshotOutput(nil, err)
			}
			defer f.Close()
			reply, err := screenshotNative(ctx, req, []*os.File{f})
			return screenshotOutput(reply.Result, err)
		}
	}
	if op == "capabilities" || op == "sources" {
		reply, err := screenshotNative(ctx, req, nil)
		if op == "capabilities" && err != nil {
			return screenshotOutput(map[string]any{"capture_available": false, "annotation_available": false, "reason": screenshot.AsError(err).Code, "mcp_required": false}, nil)
		}
		return screenshotOutput(reply.Result, err)
	}
	doc, err := screenshot.Produce(ctx, req, input, output, screenshotNative)
	if err != nil {
		return screenshotOutput(nil, err)
	}
	meta := strings.TrimSuffix(doc.ImagePath, filepath.Ext(doc.ImagePath)) + ".capture.json"
	return screenshotOutput(map[string]any{"document_path": meta, "document": doc}, nil)
}
