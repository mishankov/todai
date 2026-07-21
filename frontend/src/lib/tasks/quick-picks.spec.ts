import { afterEach, describe, expect, it } from 'vitest';
import { entityItems, localDateValue, relativeDateOptions } from './quick-picks';

const originalTimezone = process.env.TZ;

afterEach(() => {
	process.env.TZ = originalTimezone;
});

describe('relativeDateOptions', () => {
	it('crosses month and year boundaries using local calendar days', () => {
		process.env.TZ = 'Europe/Moscow';
		const options = relativeDateOptions(new Date(2026, 11, 31, 23, 30));

		expect(options.map((option) => option.value)).toEqual([
			'2026-12-31',
			'2027-01-01',
			'2027-01-04'
		]);
	});

	it('always returns Monday in the strictly following calendar week', () => {
		process.env.TZ = 'Europe/Moscow';
		const monday = relativeDateOptions(new Date(2026, 6, 20, 12))[2];
		const sunday = relativeDateOptions(new Date(2026, 6, 26, 12))[2];

		expect(monday.value).toBe('2026-07-27');
		expect(sunday.value).toBe('2026-07-27');
		expect(monday.date.getDay()).toBe(1);
		expect(sunday.date.getDay()).toBe(1);
	});

	it('does not drift across daylight-saving changes', () => {
		process.env.TZ = 'America/New_York';
		const spring = relativeDateOptions(new Date(2026, 2, 7, 23, 30));
		const autumn = relativeDateOptions(new Date(2026, 9, 31, 23, 30));

		expect(spring[1].value).toBe('2026-03-08');
		expect(spring[1].date.getHours()).toBe(0);
		expect(autumn[1].value).toBe('2026-11-01');
		expect(autumn[1].date.getHours()).toBe(0);
	});

	it('formats local values without UTC conversion', () => {
		process.env.TZ = 'Pacific/Kiritimati';
		expect(localDateValue(new Date(2027, 0, 1, 0, 15))).toBe('2027-01-01');
	});
});

describe('entityItems', () => {
	it('puts current and recent available entities before the full list', () => {
		const entities = ['one', 'two', 'three', 'four', 'five'].map((id) => ({
			id,
			name: id,
			layout: 'list' as const,
			colorTheme: 'sage' as const,
			agentModel: 'model',
			agentThinkingEffort: 'medium' as const,
			position: 0,
			version: 1,
			archivedAt: null,
			createdAt: '',
			updatedAt: '',
			lastModifiedBy: ''
		}));

		const items = entityItems(entities, 'three', ['missing', 'two', 'one']);

		expect(items.slice(0, 3).map(({ id, group }) => ({ id, group }))).toEqual([
			{ id: 'three', group: 'Current' },
			{ id: 'two', group: 'Recent' },
			{ id: 'one', group: 'Recent' }
		]);
		expect(items.some((item) => item.id === 'missing')).toBe(false);
	});
});
