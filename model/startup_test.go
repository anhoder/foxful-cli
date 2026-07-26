package model

import (
	"testing"
	"time"
)

func TestStartupRerendersDoNotAdvanceLoadingDuration(t *testing.T) {
	options := DefaultOptions()
	next := &Main{}
	startup := NewStartup(&options.StartupOptions, next)
	app := NewApp(options)
	app.page = startup

	started := time.Now()
	rerenders := int(options.LoadingDuration/options.TickDuration) + 1
	for i := range rerenders {
		msg := app.RerenderCmd(false)()
		page, _ := startup.Update(msg, app)
		if page == next {
			t.Fatalf("startup completed after %d immediate rerenders in %s, before loading duration %s", i+1, time.Since(started), options.LoadingDuration)
		}
	}
}

func TestStartupCompletesAfterLoadingDuration(t *testing.T) {
	options := DefaultOptions()
	next := &Main{}
	startup := NewStartup(&options.StartupOptions, next)
	app := NewApp(options)
	app.page = startup
	startup.startedAt = time.Now().Add(-options.LoadingDuration)

	page, _ := startup.Update(tickStartupMsg{}, app)
	if page != next {
		t.Fatalf("startup page = %T, want next page after loading duration", page)
	}
	if startup.loadedDuration != options.LoadingDuration {
		t.Errorf("loaded duration = %s, want %s", startup.loadedDuration, options.LoadingDuration)
	}
}
