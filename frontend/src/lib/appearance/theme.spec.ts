import { describe, expect, it } from 'vitest';
import {
	appearanceCacheKey,
	cacheAppearance,
	cachedAppearance,
	effectiveAppearance
} from './theme';

describe('appearance theme', () => {
	it('follows system changes only in system mode', () => {
		expect(effectiveAppearance('system', false)).toBe('light');
		expect(effectiveAppearance('system', true)).toBe('dark');
		expect(effectiveAppearance('light', true)).toBe('light');
		expect(effectiveAppearance('dark', false)).toBe('dark');
	});

	it('accepts only valid cached preferences and replaces cache with the saved server value', () => {
		const values = new Map<string, string>([[appearanceCacheKey, 'sepia']]);
		const storage = {
			getItem: (key: string) => values.get(key) ?? null,
			setItem: (key: string, value: string) => values.set(key, value)
		};
		expect(cachedAppearance(storage)).toBeNull();
		cacheAppearance(storage, 'dark');
		expect(cachedAppearance(storage)).toBe('dark');
	});
});
