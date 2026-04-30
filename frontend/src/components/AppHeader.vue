<script setup lang="ts">
import { reactive, ref, onMounted, onUnmounted } from "vue";

import { formatDateTime } from "../format";
import { useI18n } from "../i18n";
import { isWindowMaximised, maximiseWindow, restoreWindow, startWindowDrag, minimiseWindow, closeWindow } from "../wails-runtime";
import type { StatusTone } from "../types";

defineProps<{
    statusText: string;
    statusTone: StatusTone;
    generatedAt: string;
}>();

const { t } = useI18n();

const isMaximised = ref(false);

async function checkMaximised() {
    isMaximised.value = await isWindowMaximised();
}

onMounted(() => {
    checkMaximised();
    // Poll or use an event if Wails v3 provides window events to JS
    const timer = setInterval(checkMaximised, 1000);
    onUnmounted(() => clearInterval(timer));
});

const dragState = reactive({
    active: false,
    dragging: false,
    startX: 0,
    startY: 0,
});
let dragResetTimer = 0;

function isInteractiveTarget(target: EventTarget | null): boolean {
    if (!(target instanceof Element)) {
        return false;
    }

    return Boolean(target.closest("button, a, input, textarea, select, [role='button'], [data-window-dblclick-ignore='true']"));
}

function resetDragState(): void {
    if (dragResetTimer !== 0) {
        window.clearTimeout(dragResetTimer);
        dragResetTimer = 0;
    }
    dragState.active = false;
    dragState.dragging = false;
}

function scheduleDragStateReset(): void {
    if (dragResetTimer !== 0) {
        window.clearTimeout(dragResetTimer);
    }
    dragResetTimer = window.setTimeout(() => {
        resetDragState();
    }, 800);
}

function handleBarMouseDown(event: MouseEvent): void {
    if (isInteractiveTarget(event.target)) {
        return;
    }

    dragState.active = true;
    dragState.dragging = false;
    dragState.startX = event.clientX;
    dragState.startY = event.clientY;
}

function handleBarMouseMove(event: MouseEvent): void {
    if (!dragState.active || dragState.dragging || isInteractiveTarget(event.target)) {
        return;
    }

    const movedX = Math.abs(event.clientX - dragState.startX);
    const movedY = Math.abs(event.clientY - dragState.startY);
    if (movedX < 4 && movedY < 4) {
        return;
    }

    dragState.dragging = true;
    startWindowDrag();
    scheduleDragStateReset();
}

function handleBarMouseUp(): void {
    resetDragState();
}

function handleBarMouseLeave(): void {
    resetDragState();
}

// Toggle window maximize/restore on title bar double-click, avoiding reliance on macOS default behavior for custom drag regions.
async function handleBarDoubleClick(event: MouseEvent): Promise<void> {
    if (dragState.dragging || isInteractiveTarget(event.target)) {
        return;
    }

    event.preventDefault();
    if (isMaximised.value) {
        restoreWindow();
    } else {
        maximiseWindow();
    }
    checkMaximised();
}

function handleMinimise() {
    minimiseWindow();
}

function handleMaximiseToggle() {
    if (isMaximised.value) {
        restoreWindow();
    } else {
        maximiseWindow();
    }
    checkMaximised();
}

function handleClose() {
    closeWindow();
}
</script>

<template>
    <header class="window-bar" @mousedown="handleBarMouseDown" @mousemove="handleBarMouseMove" @mouseup="handleBarMouseUp" @mouseleave="handleBarMouseLeave" @dblclick="handleBarDoubleClick">
        <div class="window-bar-spacer" aria-hidden="true"></div>
        <div class="window-tools">
            <div class="window-status" :data-tone="statusTone">
                <span class="window-status-text">{{ statusText }}</span>
                <span class="window-status-separator">·</span>
                <span class="window-status-time">{{ t("app.recentRefresh", { time: formatDateTime(generatedAt) }) }}</span>
            </div>
            <div class="window-controls">
                <button type="button" class="control-btn" :title="t('app.window.minimise')" @click.stop="handleMinimise">
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><path d="M2 6h8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>
                </button>
                <button type="button" class="control-btn" :title="isMaximised ? t('app.window.restore') : t('app.window.maximise')" @click.stop="handleMaximiseToggle">
                    <svg v-if="isMaximised" width="12" height="12" viewBox="0 0 12 12" fill="none"><rect x="3.5" y="3.5" width="6" height="6" rx="1" stroke="currentColor" stroke-width="1.2"/><path d="M2.5 8.5V3.5a1 1 0 011-1h5" stroke="currentColor" stroke-width="1.2"/></svg>
                    <svg v-else width="12" height="12" viewBox="0 0 12 12" fill="none"><rect x="2.5" y="2.5" width="7" height="7" rx="1" stroke="currentColor" stroke-width="1.2"/></svg>
                </button>
                <button type="button" class="control-btn close-btn" :title="t('app.window.close')" @click.stop="handleClose">
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><path d="M3 3l6 6M9 3L3 9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>
                </button>
            </div>
        </div>
    </header>
</template>

<style scoped>
.window-bar {
    min-height: 40px;
    padding: 4px 10px 2px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    cursor: default;
    user-select: none;
    -webkit-user-select: none;
}

.window-bar-spacer {
    flex: 1 1 auto;
    min-width: 0;
}

.window-tools {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
}

.window-status {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 2px 4px;
    border: none;
    background: none;
}

.window-status-text,
.window-status-time {
    max-width: min(42vw, 520px);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.window-status-text {
    font-size: 10px;
    color: var(--muted);
}

.window-status-time {
    font-size: 10px;
    color: var(--muted);
}

.window-status-separator {
    font-size: 10px;
    color: var(--muted);
    opacity: 0.7;
}

.window-status[data-tone="error"] .window-status-text {
    color: var(--fall);
}

.window-status[data-tone="warn"] .window-status-text {
    color: var(--warn);
}

.window-status[data-tone="success"] .window-status-text {
    color: var(--accent);
}

.window-controls {
    display: flex;
    align-items: center;
    margin-left: 8px;
    height: 32px;
}

.control-btn {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    transition: all 0.2s ease;
    border-radius: 6px;
}

.control-btn:hover {
    background: rgba(0, 0, 0, 0.05);
    color: var(--ink);
}

.app-dark .control-btn:hover,
html[data-theme="dark"] .control-btn:hover {
    background: rgba(255, 255, 255, 0.1);
}

.control-btn.close-btn:hover {
    background: #e81123;
    color: white;
}

@media (max-width: 880px) {
    .window-bar {
        padding-left: 12px;
    }

    .window-tools {
        align-items: flex-end;
        flex-direction: column;
    }

    .window-status {
        flex-wrap: wrap;
        justify-content: flex-end;
    }
}
</style>
