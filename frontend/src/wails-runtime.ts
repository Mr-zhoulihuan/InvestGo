import { api } from "./api";

type WailsWindowRuntime = {
    WindowIsMaximised(): Promise<boolean>;
    WindowMaximise(): void;
    WindowUnmaximise(): void;
    WindowMinimise(): void;
    WindowClose(): void;
    WindowRestore(): void;
};

type WailsBridge = {
    invoke(message: string): void;
};

function getWailsBridge(): WailsBridge | null {
    return (window as Window & { _wails?: WailsBridge })._wails ?? null;
}

// Ensure the runtime object is mapped correctly for Wails v3 alpha
function getRuntimeMethods() {
    const w = window as any;
    const wails = w.wails;
    const runtime = w.runtime;

    // Try Wails v3 Window API (capitalised or lowercase)
    const winApi = wails?.Window || wails?.window;
    if (winApi) {
        return {
            IsMaximised: winApi.IsMaximised || winApi.WindowIsMaximised || winApi.isMaximised,
            Maximise: winApi.Maximise || winApi.WindowMaximise || winApi.maximise,
            Unmaximise: winApi.Unmaximise || winApi.WindowUnmaximise || winApi.unmaximise,
            Minimise: winApi.Minimise || winApi.WindowMinimise || winApi.minimise,
            Close: winApi.Close || winApi.WindowClose || winApi.close,
            Restore: winApi.Restore || winApi.WindowRestore || winApi.restore || winApi.Unmaximise || winApi.WindowUnmaximise || winApi.unmaximise
        };
    }

    // Fallback to legacy runtime object or window level methods if present
    const r = runtime || w;
    return {
        IsMaximised: r.WindowIsMaximised || r.IsMaximised || r.isMaximised,
        Maximise: r.WindowMaximise || r.Maximise || r.maximise,
        Unmaximise: r.WindowUnmaximise || r.Unmaximise || r.unmaximise,
        Minimise: r.WindowMinimise || r.Minimise || r.minimise,
        Close: r.WindowClose || r.Close || r.close,
        Restore: r.WindowRestore || r.Restore || r.restore || r.WindowUnmaximise || r.Unmaximise || r.unmaximise
    };
}

// The frontend can also run under the browser dev server, so the Wails runtime is safely wrapped here.
export async function isWindowMaximised(): Promise<boolean> {
    const methods = getRuntimeMethods();
    if (!methods || !methods.IsMaximised) return false;
    try {
        return await methods.IsMaximised();
    } catch {
        return false;
    }
}

async function invokeWindowAction(action: "minimise" | "maximise" | "unmaximise" | "close"): Promise<void> {
    const methods = getRuntimeMethods();
    let success = false;

    // Try native JS API first
    if (methods) {
        try {
            switch (action) {
                case "minimise":
                    if (methods.Minimise) {
                        methods.Minimise();
                        success = true;
                    }
                    break;
                case "maximise":
                    if (methods.Maximise) {
                        methods.Maximise();
                        success = true;
                    }
                    break;
                case "unmaximise":
                    if (methods.Restore) {
                        methods.Restore();
                        success = true;
                    }
                    break;
                case "close":
                    if (methods.Close) {
                        methods.Close();
                        success = true;
                    }
                    break;
            }
        } catch (e) {
            console.warn(`Native window action ${action} failed:`, e);
        }
    }

    // If native API failed or is missing, fallback to our custom Go API
    if (!success) {
        try {
            await api("/api/window", {
                method: "POST",
                body: JSON.stringify({ action }),
            });
        } catch (e) {
            console.error(`Fallback window action ${action} failed:`, e);
        }
    }
}

// Maximize the window to the current available workspace.
export function maximiseWindow(): void {
    invokeWindowAction("maximise");
}

// Restore the window from maximized state to its original size.
export function restoreWindow(): void {
    invokeWindowAction("unmaximise");
}

// Minimise the window.
export function minimiseWindow(): void {
    invokeWindowAction("minimise");
}

// Close the window.
export function closeWindow(): void {
    invokeWindowAction("close");
}

// Trigger native Wails window dragging.
export function startWindowDrag(): void {
    getWailsBridge()?.invoke("wails:drag");
}
