package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"investgo/internal/api"
	"investgo/internal/core"
	"investgo/internal/core/hot"
	"investgo/internal/core/marketdata"
	"investgo/internal/core/store"
	"investgo/internal/logger"
	"investgo/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var defaultTerminalLogging = "0"
var defaultDevToolsBuild = "0"
var appVersion = "dev"

// Embed frontend build assets for Wails to serve as static resources at runtime.
//
//go:embed frontend/dist
var frontendAssets embed.FS

// Embed application icon
//
//go:embed build/appicon.png
var appIcon []byte

func main() {
	logs := logger.NewLogBook(400)
	if terminalLoggingEnabled() {
		logs.EnableConsole(os.Stderr)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(logs.Writer("backend", "stdlib", logger.DeveloperLogError))
	if err := logs.ConfigureFile(defaultLogPath()); err != nil {
		log.Printf("configure log file: %v", err)
	}
	defer func() { _ = logs.Close() }()

	logs.Info("backend", "app", "starting InvestGo")

	// Bootstrap the shared HTTP transport with the "system" default so that
	// HTTP clients are available before the Store is initialised. The transport
	// will be updated to the actual persisted setting once the Store is ready.
	proxyTransport := platform.NewProxyTransport("system", "")
	httpClient := platform.NewHTTPClient(proxyTransport)

	var settingsFunc func() core.AppSettings = func() core.AppSettings { return core.AppSettings{} }
	registry := marketdata.DefaultRegistry(httpClient, func() core.AppSettings { return settingsFunc() })

	store, err := store.NewStore(
		defaultStatePath(),
		registry.QuoteProviders(),
		registry.QuoteSourceOptions(),
		registry.NewHistoryRouter(func() core.AppSettings { return settingsFunc() }),
		logs,
		appVersion,
		httpClient, // shared http.Client so FX rate requests respect the configured proxy transport
	)
	if err != nil {
		log.Fatalf("initialise store: %v", err)
	}

	// Wire the real settings getter now that the Store is ready.
	settingsFunc = store.CurrentSettings

	// The Store is now loaded — sync the proxy transport with the persisted
	// settings. ApplySystemProxy sets process-wide env vars so that
	// http.ProxyFromEnvironment works correctly for "system" mode.
	snapshot := store.Snapshot()
	proxyMode := snapshot.Settings.ProxyMode
	proxyURL := snapshot.Settings.ProxyURL
	logs.Info("backend", "proxy", fmt.Sprintf("proxy mode: %s", proxyMode))
	if proxyMode == "system" {
		platform.ApplySystemProxy(logs)
	} else if proxyMode == "custom" && proxyURL != "" {
		logs.Info("backend", "proxy", fmt.Sprintf("custom proxy: %s", proxyURL))
	}
	proxyTransport.Update(proxyMode, proxyURL)

	hotService := hot.NewHotService(httpClient, logs.NewSlogLogger("hot", slog.LevelInfo), registry)

	frontendFS, err := fs.Sub(frontendAssets, "frontend/dist")
	if err != nil {
		log.Fatalf("load frontend assets: %v", err)
	}

	// We need a placeholder app instance to create the API handler,
	// but the handler needs the real app instance. Since they circularly depend
	// on each other during bootstrap, we'll create the handler and then the app.
	var app *application.App

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		// This lazy-wires the handler to use the app instance once it's created.
		api.NewHandler(store, hotService, logs, proxyTransport, app).ServeHTTP(w, r)
	})
	mux.Handle("/", application.BundledAssetFileServer(frontendFS))

	app = application.New(application.Options{
		Name:        "InvestGo",
		Description: "Go + Wails v3 Investment Monitor Desktop App",
		Icon:        appIcon,
		Logger:      logs.NewSlogLogger("system", slog.LevelInfo),
		Assets: application.AssetOptions{
			Handler:        mux,
			DisableLogging: true,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		PanicHandler: func(details *application.PanicDetails) {
			logs.Error("backend", "panic", fmt.Sprintf("%s\n%s", details.Error, details.StackTrace))
		},
		OnShutdown: func() {
			logs.Info("backend", "app", "shutdown requested")
			if err := store.Save(); err != nil {
				logs.Error("backend", "storage", fmt.Sprintf("save state on shutdown failed: %v", err))
			}
		},
	})

	useNativeTitleBar := snapshot.Settings.UseNativeTitleBar

	// Calculate initial window size based on primary screen resolution
	width, height := 1200, 828 // Default fallback
	if primaryScreen := app.Screen.GetPrimary(); primaryScreen != nil {
		// Calculate 80% of screen width/height, but not exceeding a reasonable maximum
		targetWidth := int(float64(primaryScreen.Size.Width) * 0.8)
		targetHeight := int(float64(primaryScreen.Size.Height) * 0.8)

		// Clamp to sensible defaults for a desktop app
		if targetWidth > 1600 {
			targetWidth = 1600
		}
		if targetHeight > 1000 {
			targetHeight = 1000
		}
		// Ensure it's not smaller than our MinWidth/MinHeight (match window.go)
		if targetWidth < 1024 {
			targetWidth = 1024
		}
		if targetHeight < 700 {
			targetHeight = 700
		}
		width, height = targetWidth, targetHeight
	}

	windowOptions := platform.BuildMainWindowOptions(useNativeTitleBar, width, height)
	windowOptions.KeyBindings = map[string]func(window application.Window){
		"F12": func(window application.Window) {
			snapshot := store.Snapshot()
			if !snapshot.Settings.DeveloperMode {
				logs.Warn("system", "devtools", "ignored F12 because developer mode is disabled")
				return
			}
			if !devToolsBuildEnabled() {
				logs.Warn("system", "devtools", "ignored F12 because this binary was built without devtools support")
				return
			}
			logs.Info("system", "devtools", "opening web inspector")
			window.OpenDevTools()
		},
	}

	app.Window.NewWithOptions(windowOptions)

	if err := app.Run(); err != nil {
		log.Printf("run app: %v", err)
		os.Exit(1)
	}
}

// defaultStatePath returns the default storage path for the state file.
func defaultStatePath() string {
	if configDir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(configDir, "investgo", "state.json")
	}

	return filepath.Join(".", "data", "state.json")
}

// defaultLogPath returns the default storage path for the log file.
// Log files and state.json are located at $HOME/Library/Application Support/investgo/.
func defaultLogPath() string {
	if configDir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(configDir, "investgo", "logs", "app.log")
	}

	return filepath.Join(".", "data", "logs", "app.log")
}

// terminalLoggingEnabled returns whether the current process should output development logs to the terminal.
func terminalLoggingEnabled() bool {
	if defaultTerminalLogging == "1" {
		return true
	}

	for _, arg := range os.Args[1:] {
		if arg == "-dev" || arg == "--dev" {
			return true
		}
	}

	return false
}

// devToolsBuildEnabled returns whether the current binary has DevTools support enabled.
func devToolsBuildEnabled() bool {
	return defaultDevToolsBuild == "1"
}
