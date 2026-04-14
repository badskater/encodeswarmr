// Package page contains one file per application screen. Each page implements
// the nav.Page interface (OnNavigatedTo, OnNavigatedFrom, Layout). The
// register.go file wires all page factories to the router at startup.
package page

import (
	"log/slog"

	"gioui.org/app"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
	"github.com/badskater/encodeswarmr/internal/desktop/page/admin"
)

// RegisterAll registers all page factories with the router.
// Real pages close over state, router, window, and logger directly.
// Placeholder pages are registered for all routes not yet implemented.
func RegisterAll(router *nav.Router, state *desktopapp.State, w *app.Window, logger *slog.Logger) {
	router.Register("/login", func() nav.Page {
		return NewLoginPage(state, router, w, logger)
	})

	router.Register("/dashboard", func() nav.Page {
		return NewDashboardPage(state, router, w, logger)
	})

	// Core pages
	router.Register("/jobs", func() nav.Page {
		return NewJobsPage(state, router, w, logger)
	})
	router.Register("/jobs/detail", func() nav.Page {
		return NewJobDetailPage(state, router, w, logger)
	})
	router.Register("/sources", func() nav.Page {
		return NewSourcesPage(state, router, w, logger)
	})
	router.Register("/sources/detail", func() nav.Page {
		return NewSourceDetailPage(state, router, w, logger)
	})
	router.Register("/agents", func() nav.Page {
		return NewAgentsPage(state, router, w, logger)
	})
	router.Register("/agents/detail", func() nav.Page {
		return NewAgentDetailPage(state, router, w, logger)
	})
	router.Register("/tasks/detail", func() nav.Page {
		return NewTaskDetailPage(state, router, w, logger)
	})

	// Admin pages
	router.Register("/admin/templates", func() nav.Page {
		return admin.NewTemplatesPage(state, router, w, logger)
	})
	router.Register("/admin/variables", func() nav.Page {
		return admin.NewVariablesPage(state, router, w, logger)
	})
	router.Register("/admin/webhooks", func() nav.Page {
		return admin.NewWebhooksPage(state, router, w, logger)
	})
	router.Register("/admin/users", func() nav.Page {
		return admin.NewUsersPage(state, router, w, logger)
	})
	router.Register("/admin/api-keys", func() nav.Page {
		return admin.NewAPIKeysPage(state, router, w, logger)
	})
	router.Register("/admin/agent-pools", func() nav.Page {
		return admin.NewAgentPoolsPage(state, router, w, logger)
	})
	router.Register("/admin/path-mappings", func() nav.Page {
		return admin.NewPathMappingsPage(state, router, w, logger)
	})
	router.Register("/admin/tokens", func() nav.Page {
		return admin.NewTokensPage(state, router, w, logger)
	})
	router.Register("/admin/schedules", func() nav.Page {
		return admin.NewSchedulesPage(state, router, w, logger)
	})
	router.Register("/admin/plugins", func() nav.Page {
		return admin.NewPluginsPage(state, router, w, logger)
	})
	router.Register("/admin/encoding-rules", func() nav.Page {
		return admin.NewEncodingRulesPage(state, router, w, logger)
	})
	router.Register("/admin/encoding-profiles", func() nav.Page {
		return admin.NewEncodingProfilesPage(state, router, w, logger)
	})
	router.Register("/admin/watch-folders", func() nav.Page {
		return admin.NewWatchFoldersPage(state, router, w, logger)
	})
	router.Register("/admin/auto-scaling", func() nav.Page {
		return admin.NewAutoScalingPage(state, router, w, logger)
	})
	router.Register("/admin/notifications", func() nav.Page {
		return admin.NewNotificationsPage(state, router, w, logger)
	})
	router.Register("/admin/audit-export", func() nav.Page {
		return admin.NewAuditExportPage(state, router, w, logger)
	})

	// Remaining operational pages
	router.Register("/queue", func() nav.Page {
		return NewQueuePage(state, router, w, logger)
	})
	router.Register("/audio", func() nav.Page {
		return NewAudioPage(state, router, w, logger)
	})
	router.Register("/flows", func() nav.Page {
		return NewFlowsPage(state, router, w, logger)
	})
	router.Register("/files", func() nav.Page {
		return NewFileManagerPage(state, router, w, logger)
	})
	router.Register("/sessions", func() nav.Page {
		return NewSessionsPage(state, router, w, logger)
	})
}
