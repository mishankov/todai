import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import AppearanceController from './AppearanceController.svelte';
import { appearanceCacheKey } from './theme';

describe('AppearanceController', () => {
	afterEach(() => {
		vi.restoreAllMocks();
		localStorage.removeItem(appearanceCacheKey);
		delete document.documentElement.dataset.appearance;
		document.documentElement.style.colorScheme = '';
	});

	it('tracks media-query changes in system mode and reconciles the cache with the server value', async () => {
		const media = mockMediaQuery(false);
		vi.spyOn(window, 'matchMedia').mockReturnValue(media.query);
		localStorage.setItem(appearanceCacheKey, 'dark');

		render(AppearanceController, { appearance: 'system' });
		await vi.waitFor(() => expect(document.documentElement.dataset.appearance).toBe('light'));
		expect(localStorage.getItem(appearanceCacheKey)).toBe('system');

		media.setDark(true);
		await vi.waitFor(() => expect(document.documentElement.dataset.appearance).toBe('dark'));
	});

	it('ignores media-query changes in a forced mode', async () => {
		const media = mockMediaQuery(true);
		vi.spyOn(window, 'matchMedia').mockReturnValue(media.query);

		render(AppearanceController, { appearance: 'light' });
		await vi.waitFor(() => expect(document.documentElement.dataset.appearance).toBe('light'));

		media.setDark(false);
		await vi.waitFor(() => expect(document.documentElement.dataset.appearance).toBe('light'));
	});
});

function mockMediaQuery(initialDark: boolean): {
	query: MediaQueryList;
	setDark: (dark: boolean) => void;
} {
	let dark = initialDark;
	const listeners = new Set<(event: MediaQueryListEvent) => void>();
	const query = {
		media: '(prefers-color-scheme: dark)',
		get matches() {
			return dark;
		},
		onchange: null,
		addEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) =>
			listeners.add(listener),
		removeEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) =>
			listeners.delete(listener),
		addListener: () => undefined,
		removeListener: () => undefined,
		dispatchEvent: () => true
	} as MediaQueryList;
	return {
		query,
		setDark(next) {
			dark = next;
			const event = { matches: next, media: query.media } as MediaQueryListEvent;
			for (const listener of listeners) listener(event);
		}
	};
}
