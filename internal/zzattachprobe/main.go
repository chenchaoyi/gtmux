// LOCAL probe only (never committed): opens /api/attach as a given token, optionally types,
// and reports which synthetic markers appeared in the output and when the stream ended.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/chenchaoyi/gtmux/internal/connect"
)

func main() {
	base := flag.String("base", "http://127.0.0.1:18995", "")
	token := flag.String("token", "", "")
	pane := flag.String("pane", "%0", "")
	send := flag.String("send", "", "bytes to type after 1s (Go-quoted)")
	dur := flag.Duration("for", 4*time.Second, "")
	after := flag.Duration("after", 0, "also count output bytes received after this point")
	markers := flag.String("markers", "ALLOWED_PANE_SYNTH,UNSHARED_PANE_SYNTH,OTHER_SESSION_SYNTH", "")
	flag.Parse()
	u := strings.Replace(*base, "http", "ws", 1) + "/api/attach?id=" + url.QueryEscape(*pane) + "&term=xterm-256color"
	conn, resp, err := websocket.DefaultDialer.Dial(u, http.Header{"Authorization": {"Bearer " + *token}})
	if err != nil {
		code := 0
		body := ""
		if resp != nil {
			code = resp.StatusCode
			b := make([]byte, 512)
			n, _ := resp.Body.Read(b)
			body = strings.TrimSpace(string(b[:n]))
		}
		fmt.Printf("RESULT upgrade=refused http=%d body=%s\n", code, body)
		return
	}
	_ = conn.WriteMessage(websocket.BinaryMessage, connect.EncodeResize(120, 30))
	start := time.Now()
	var out strings.Builder
	first := map[string]float64{}
	ended := -1.0
	late := 0
	endWhy := ""
	go func() {
		if *send == "" {
			return
		}
		time.Sleep(time.Second)
		s, err := unquote(*send)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad -send:", err)
			return
		}
		for _, part := range strings.Split(s, "\x00") { // \x00 = pause between keystroke groups
			_ = conn.WriteMessage(websocket.BinaryMessage, connect.Encode(connect.OpInput, []byte(part)))
			time.Sleep(300 * time.Millisecond)
		}
	}()
	_ = conn.SetReadDeadline(start.Add(*dur))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if time.Since(start) < *dur-50*time.Millisecond {
				ended = time.Since(start).Seconds()
				endWhy = err.Error()
			}
			break
		}
		op, payload, ok := connect.Decode(data)
		if !ok || op != connect.OpOutput {
			continue
		}
		out.WriteString(string(payload))
		if *after > 0 && time.Since(start) > *after {
			late += len(payload)
		}
		for _, m := range strings.Split(*markers, ",") {
			if _, seen := first[m]; !seen && strings.Contains(out.String(), m) {
				first[m] = time.Since(start).Seconds()
			}
		}
	}
	fmt.Printf("RESULT upgrade=ok bytes=%d", out.Len())
	for _, m := range strings.Split(*markers, ",") {
		if t, ok := first[m]; ok {
			fmt.Printf(" %s@%.1fs", m, t)
		} else {
			fmt.Printf(" %s=absent", m)
		}
	}
	if *after > 0 {
		fmt.Printf(" bytes-after-%s=%d", after.String(), late)
	}
	if strings.Contains(out.String(), "access revoked") {
		fmt.Printf(" revoked-notice=yes")
	}
	if ended >= 0 {
		fmt.Printf(" stream-ended@%.1fs(%s)", ended, endWhy)
	} else {
		fmt.Printf(" stream-open-until-%s", dur.String())
	}
	fmt.Println()
	if os.Getenv("PROBE_DUMP") != "" {
		_ = os.WriteFile(os.Getenv("PROBE_DUMP"), []byte(out.String()), 0o600)
	}
}

func unquote(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && s[i+1] == 'x' {
			var v byte
			if _, err := fmt.Sscanf(s[i+2:i+4], "%02x", &v); err != nil {
				return "", err
			}
			b.WriteByte(v)
			i += 3
			continue
		}
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == 'r' {
			b.WriteByte('\r')
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String(), nil
}
