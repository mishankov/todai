import { describe, expect, it } from 'vitest';
import type { Project } from './client';
import {
	accountDestinationPath,
	activeProjectFromLocation,
	projectIdFromLocation
} from './navigation';

describe('project navigation context', () => {
	it('builds encoded contextual account destinations without changing global destinations', () => {
		expect(accountDestinationPath('/projects', 'work/with space')).toBe(
			'/projects?project=work%2Fwith+space'
		);
		expect(accountDestinationPath('/settings', 'work/with space')).toBe(
			'/settings?project=work%2Fwith+space'
		);
		expect(accountDestinationPath('/projects')).toBe('/projects');
		expect(accountDestinationPath('/settings')).toBe('/settings');
	});

	it('resolves path and query contexts consistently', () => {
		expect(projectIdFromLocation('/projects/work%2Fwith%20space/tasks')).toBe('work/with space');
		expect(projectIdFromLocation('/projects', '?project=work%2Fwith+space')).toBe(
			'work/with space'
		);
		expect(projectIdFromLocation('/settings', '?project=work%2Fwith+space')).toBe(
			'work/with space'
		);
		expect(projectIdFromLocation('/today', '?project=work')).toBeUndefined();
	});

	it('activates only an owned active project and never remembers one for global entry', () => {
		const active = testProject({ id: 'active' });
		const archived = testProject({ id: 'archived', archivedAt: '2026-07-23T10:00:00Z' });
		const projects = [active, archived];

		expect(activeProjectFromLocation(projects, '/settings', '?project=active')).toBe(active);
		expect(activeProjectFromLocation(projects, '/projects', '?project=missing')).toBeUndefined();
		expect(activeProjectFromLocation(projects, '/projects', '?project=archived')).toBeUndefined();
		expect(activeProjectFromLocation(projects, '/settings')).toBeUndefined();
		expect(activeProjectFromLocation(projects, '/projects')).toBeUndefined();
		expect(activeProjectFromLocation(projects, '/projects/%E0%A4%A')).toBeUndefined();
	});
});

function testProject(overrides: Partial<Project> = {}): Project {
	return {
		id: 'project-id',
		name: 'Project',
		layout: 'list',
		colorTheme: 'sage',
		agentModel: 'gpt-default',
		agentThinkingEffort: 'medium',
		position: 1024,
		version: 1,
		archivedAt: null,
		createdAt: '2026-07-23T10:00:00Z',
		updatedAt: '2026-07-23T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}
