package main

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
)

// DemoPage implements a simple page that demonstrates the notification system.
type DemoPage struct {
	instructions string
}

func NewDemoPage() *DemoPage {
	return &DemoPage{
		instructions: `Notification System Demo

Keyboard shortcuts:
  1 - Info notification (auto-dismiss 4s)
  2 - Success notification (auto-dismiss 4s)
  3 - Warning notification (manual dismiss)
  4 - Error notification (manual dismiss)
  5 - Long message notification (truncated to 5 lines)
  6 - Progress update demo (shows update flow)
  7 - Spam 10 notifications (test stacking limit)
  c - Clear all notifications
  q - Quit

Click any notification to dismiss it.
Notifications stack in the top-right corner.`,
	}
}

func (p *DemoPage) Type() model.PageType {
	return "demo"
}

func (p *DemoPage) Init(app *model.App) tea.Cmd {
	// Show a welcome notification on startup
	app.Notify(model.NotificationSpec{
		Level:   model.NotificationInfo,
		Title:   "Welcome",
		Message: "Press 1-7 to test notifications, c to clear all",
		Timeout: 5 * time.Second,
	})
	return nil
}

func (p *DemoPage) Update(msg tea.Msg, app *model.App) (model.Page, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "1":
			app.Notify(model.NotificationSpec{
				Level:   model.NotificationInfo,
				Title:   "Info",
				Message: "This is an informational message that will auto-dismiss in 4 seconds.",
			})
		case "2":
			app.Notify(model.NotificationSpec{
				Level:   model.NotificationSuccess,
				Title:   "Success",
				Message: "Operation completed successfully! This will auto-dismiss.",
			})
		case "3":
			app.Notify(model.NotificationSpec{
				Level:   model.NotificationWarning,
				Title:   "Warning",
				Message: "This is a warning that requires manual dismissal. Click it or press 'c' to clear.",
				Timeout: 0, // Manual dismiss
			})
		case "4":
			app.Notify(model.NotificationSpec{
				Level:   model.NotificationError,
				Title:   "Error",
				Message: "An error occurred. This notification persists until dismissed.",
				Timeout: 0,
			})
		case "5":
			longMsg := `This is a very long message to demonstrate text wrapping and truncation.
Line 2: The notification system automatically wraps text to fit the max width.
Line 3: If the message exceeds the configured max lines (default 5), it will be truncated.
Line 4: Additional lines beyond the limit will not be visible.
Line 5: This should be the last visible line with an ellipsis.
Line 6: This line should be hidden.
Line 7: This line should also be hidden.`
			app.Notify(model.NotificationSpec{
				Level:   model.NotificationInfo,
				Title:   "Long Message",
				Message: longMsg,
				Timeout: 10 * time.Second,
			})
		case "6":
			// Progress update demo
			id := app.Notify(model.NotificationSpec{
				Level:   model.NotificationInfo,
				Title:   "Downloading",
				Message: "Progress: 0%",
				Timeout: 0,
			})
			
			// Simulate progress updates
			go func() {
				for i := 1; i <= 10; i++ {
					time.Sleep(500 * time.Millisecond)
					app.UpdateNotification(id, model.NotificationSpec{
						Level:   model.NotificationInfo,
						Title:   "Downloading",
						Message: fmt.Sprintf("Progress: %d%%", i*10),
						Timeout: 0,
					})
				}
				// Final success notification
				time.Sleep(500 * time.Millisecond)
				app.UpdateNotification(id, model.NotificationSpec{
					Level:   model.NotificationSuccess,
					Title:   "Complete",
					Message: "Download finished successfully!",
					Timeout: 3 * time.Second,
				})
			}()
		case "7":
			// Spam notifications to test stacking limit
			for i := 1; i <= 10; i++ {
				app.Notify(model.NotificationSpec{
					Level:   model.NotificationInfo,
					Message: fmt.Sprintf("Notification #%d", i),
					Timeout: 0,
				})
			}
		case "c", "C":
			app.ClearAllNotifications()
		}
	}
	return p, nil
}

func (p *DemoPage) View(app *model.App) string {
	return p.instructions
}

func (p *DemoPage) Msg() tea.Msg {
	return nil
}

func (p *DemoPage) IgnoreQuitKeyMsg(tea.KeyMsg) bool {
	return false
}

func main() {
	opts := model.DefaultOptions()
	opts.AppName = "Notification Demo"
	opts.AltScreen = true
	opts.InitPage = NewDemoPage()

	// Customize notification options (optional)
	opts.NotificationOptions.Anchor = model.AnchorTopRight
	opts.NotificationOptions.DefaultTimeout = 4 * time.Second
	opts.NotificationOptions.MaxLines = 5
	opts.NotificationOptions.Gap = 1

	app := model.NewApp(opts)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
