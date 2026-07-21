import { describe, expect, it, vi } from 'vitest';
import { SettingsConflictError, getSettings, updateSettings, type SettingsView } from './client';

describe('settings client', () => {
	it('loads and updates user settings', async () => {
		const view = testSettingsView();
		const fetcher = vi.fn(
			async () =>
				new Response(JSON.stringify(view), {
					status: 200,
					headers: { 'Content-Type': 'application/json' }
				})
		) as unknown as typeof fetch;

		await expect(getSettings(fetcher)).resolves.toEqual(view);
		await expect(
			updateSettings(fetcher, {
				timezone: 'Europe/Moscow',
				agentModel: 'gpt-fast',
				agentThinkingEffort: 'high',
				version: 1
			})
		).resolves.toEqual(view);
		expect(fetcher).toHaveBeenLastCalledWith(
			'/api/settings',
			expect.objectContaining({
				method: 'PATCH',
				body: JSON.stringify({
					timezone: 'Europe/Moscow',
					agentModel: 'gpt-fast',
					agentThinkingEffort: 'high',
					version: 1
				})
			})
		);
	});

	it('reports optimistic concurrency conflicts', async () => {
		const fetcher = vi.fn(
			async () => new Response(null, { status: 409 })
		) as unknown as typeof fetch;
		await expect(
			updateSettings(fetcher, {
				timezone: 'UTC',
				agentModel: 'gpt-fast',
				agentThinkingEffort: 'medium',
				version: 1
			})
		).rejects.toBeInstanceOf(SettingsConflictError);
	});

	it('normalizes missing model choices to empty lists', async () => {
		const view = testSettingsView();
		const fetcher = vi.fn(
			async () =>
				new Response(
					JSON.stringify({
						...view,
						availableAgentModels: null,
						availableAgentThinkingEfforts: null
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
		) as unknown as typeof fetch;

		await expect(getSettings(fetcher)).resolves.toEqual({
			...view,
			availableAgentModels: [],
			availableAgentThinkingEfforts: []
		});
	});
});

function testSettingsView(): SettingsView {
	return {
		settings: {
			timezone: 'Europe/Moscow',
			agentModel: 'gpt-fast',
			agentThinkingEffort: 'high',
			version: 1,
			createdAt: '2026-07-18T10:00:00Z',
			updatedAt: '2026-07-18T10:00:00Z',
			lastModifiedBy: 'user-id'
		},
		availableAgentModels: ['gpt-default', 'gpt-fast'],
		availableAgentThinkingEfforts: ['off', 'low', 'medium', 'high']
	};
}
