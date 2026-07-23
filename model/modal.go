package model

import tea "charm.land/bubbletea/v2"

// Modal is a unified interface for popup dialogs and context menus.
// Both are managed on the same modal stack and share event interception.
type Modal interface {
	// update handles keyboard input
	update(msg tea.Msg)

	// handleMouse handles mouse events. Returns (handled, cmd).
	// If handled=true, the modal consumed the event.
	// If handled=false, the event should pass through to underlying UI.
	handleMouse(msg tea.MouseMsg) (bool, tea.Cmd)

	// dismissed returns true if the modal should be removed from the stack
	dismissed() bool

	// dismissOutside is called when the user clicks outside the modal
	dismissOutside()

	// complete is called after dismissal to execute any result callbacks.
	// Returns (Page, tea.Cmd) for navigation/actions, or (nil, nil) if none.
	complete(app *App) (Page, tea.Cmd)

	// allowsRightClickPassthrough returns true if right-clicks outside this modal
	// should dismiss it and pass through to open a new context menu.
	// Popup returns false (traditional modal behavior); ContextMenu returns true.
	allowsRightClickPassthrough() bool
}
