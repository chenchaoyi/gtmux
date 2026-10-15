// Package hostplatform declares host capabilities. Clients consume these facts;
// feature workflows must not infer desktop support from tool names or errors.
package hostplatform

type Capabilities struct {
	DesktopFocus         bool `json:"desktop_focus"`
	DesktopNotifications bool `json:"desktop_notifications"`
	MenuBar              bool `json:"menu_bar"`
	LidControl           bool `json:"lid_control"`
}
