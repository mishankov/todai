import { describe, expect, it, vi } from 'vitest';
import { updateProject, type Project } from './client';

describe('project client', () => {
	it('saves workspace appearance and agent settings with the project', async () => {
		const project = testProject();
		const fetcher = vi.fn(
			async () =>
				new Response(JSON.stringify(project), {
					status: 200,
					headers: { 'Content-Type': 'application/json' }
				})
		) as unknown as typeof fetch;

		await updateProject(fetcher, 'project/id', {
			version: 2,
			name: 'Work',
			layout: 'board',
			colorTheme: 'ocean',
			agentModel: 'gpt-fast',
			agentThinkingEffort: 'high'
		});

		expect(fetcher).toHaveBeenCalledWith(
			'/api/projects/project%2Fid',
			expect.objectContaining({
				method: 'PATCH',
				body: JSON.stringify({
					version: 2,
					name: 'Work',
					layout: 'board',
					colorTheme: 'ocean',
					agentModel: 'gpt-fast',
					agentThinkingEffort: 'high'
				})
			})
		);
	});
});

function testProject(): Project {
	return {
		id: 'project/id',
		name: 'Work',
		layout: 'board',
		colorTheme: 'ocean',
		agentModel: 'gpt-fast',
		agentThinkingEffort: 'high',
		position: 1024,
		version: 3,
		archivedAt: null,
		createdAt: '2026-07-21T10:00:00Z',
		updatedAt: '2026-07-21T10:01:00Z',
		lastModifiedBy: 'user-id'
	};
}
