/**
 * Purpose: Builds the side-panel terminal chrome — header, action buttons, and the
 * panel shell that hosts the terminal iframe.
 * Why: This is pure DOM construction. Keeping it beside the panel's lifecycle and
 * message-handling logic made sidepanel.ts exceed the 800-line limit; the folder
 * limit then pushes it into a sub-module rather than another sibling file.
 *
 * The module owns no state: every element it creates is handed back through
 * ShellDeps setters, and every action is an injected callback. The iframe setter is
 * called from load/error handlers, which is why this is a sink rather than a plain
 * return value.
 * Docs: docs/features/feature/terminal/index.md
 */
export interface ShellDeps {
    serverUrl: string;
    onExit(): void;
    onAnnotate(): void;
    onRedraw(): void;
    onMinimize(): void;
    onClose(): void;
    createRootFolderBar(): HTMLDivElement;
    setStatusDot(el: HTMLSpanElement): void;
    setMinimizeButton(el: HTMLButtonElement): void;
    setTerminalShell(el: HTMLDivElement): void;
    setTerminalBody(el: HTMLDivElement): void;
    setWidget(el: HTMLDivElement): void;
    setIframe(el: HTMLIFrameElement | null): void;
}
export declare function createPanelShell(token: string, deps: ShellDeps): HTMLDivElement;
//# sourceMappingURL=shell.d.ts.map