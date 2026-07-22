import { describe, expect, it } from 'vitest';
import {
	cleanTaskTitle,
	detectTitleToken,
	filterTitleOptions,
	removeTitleToken
} from './rich-title';

describe('rich task title model', () => {
	it('detects each trigger only at a token boundary', () => {
		for (const [value, type] of [
			['#Work', 'project'],
			['Task /Plan', 'section'],
			['Task !High', 'priority'],
			['Task @Tomorrow', 'due']
		] as const) {
			expect(detectTitleToken(value, value.length)?.type).toBe(type);
		}
		for (const value of ['C#', 'https://example.test/path', 'person@example.test']) {
			expect(detectTitleToken(value, value.length)).toBeNull();
		}
	});

	it('filters without case sensitivity across labels and details', () => {
		const options = [
			{ id: 'work', label: 'Work planning' },
			{ id: 'home', label: 'Home', detail: 'Personal' }
		];
		expect(filterTitleOptions(options, 'PLAn')).toEqual([options[0]]);
		expect(filterTitleOptions(options, 'personal')).toEqual([options[1]]);
	});

	it('removes the selected token and returns a clean title', () => {
		const value = 'Prepare report #Work tomorrow';
		const token = detectTitleToken(value, 'Prepare report #Work'.length)!;
		expect(removeTitleToken(value, token)).toEqual({ value: 'Prepare report tomorrow', caret: 15 });
		expect(cleanTaskTitle('  Prepare   report  ')).toBe('Prepare report');
	});

	it('keeps a dismissed trigger literal until the caret leaves its token', () => {
		const value = 'Use #literal';
		const token = detectTitleToken(value, value.length)!;
		expect(detectTitleToken(value, value.length, token.start)).toBeNull();
		expect(detectTitleToken(`${value} later`, `${value} later`.length, token.start)).toBeNull();
	});
});
