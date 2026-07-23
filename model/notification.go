package model

import (
	"time"
)

// NotificationLevel defines the semantic level of a notification.
type NotificationLevel uint8

const (
	NotificationInfo NotificationLevel = iota
	NotificationSuccess
	NotificationWarning
	NotificationError
)

// NotificationID uniquely identifies an active notification.
type NotificationID uint64

// NotificationSpec defines the content and behavior of a notification.
// Message may contain ANSI-styled text. Title is optional plain single-line text.
// Timeout of 0 means the notification must be dismissed manually (default for
// Warning/Error). For Info/Success, a zero Timeout falls back to the app's
// configured default timeout.
type NotificationSpec struct {
	Level   NotificationLevel
	Title   string
	Message string
	Timeout time.Duration
}

// notificationRect is the screen-absolute bounding box of a rendered notification.
type notificationRect struct {
	x, y, w, h int
}

func (r notificationRect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// Notification is an active notification instance managed by the App.
type Notification struct {
	id        NotificationID
	spec      NotificationSpec
	createdAt time.Time

	bounds    notificationRect
	boundsSet bool
}

func (n *Notification) setBounds(x, y, w, h int) {
	n.bounds = notificationRect{x: x, y: y, w: w, h: h}
	n.boundsSet = true
}

// ---- messages ----

// ShowNotificationMsg triggers displaying a notification. It can be sent from a
// goroutine via program.Send to remain race-free in the Update loop.
type ShowNotificationMsg struct {
	Spec NotificationSpec
}

// notificationExpireMsg signals that a notification's timeout elapsed.
type notificationExpireMsg struct {
	id NotificationID
}

// updateNotificationMsg updates the content of an existing notification.
type updateNotificationMsg struct {
	id   NotificationID
	spec NotificationSpec
}

// dismissNotificationMsg dismisses a specific notification early.
type dismissNotificationMsg struct {
	id NotificationID
}

// clearAllNotificationsMsg dismisses all visible notifications.
type clearAllNotificationsMsg struct{}
