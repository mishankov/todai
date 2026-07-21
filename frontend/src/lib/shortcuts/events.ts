export const quickAddRequestEvent = 'todai:quick-add';
export const commandPaletteRequestEvent = 'todai:command-palette';
export const chatToggleRequestEvent = 'todai:toggle-chat';

export function requestQuickAdd(): void {
	window.dispatchEvent(new CustomEvent(quickAddRequestEvent));
}

export function requestChatToggle(): void {
	window.dispatchEvent(new CustomEvent(chatToggleRequestEvent));
}
