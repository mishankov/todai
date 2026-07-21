import { describe, expect, it } from 'vitest';
import type { Project, ProjectSection } from '$lib/projects/client';
import { commandRegistry } from '$lib/shortcuts/registry';
import type { Task } from '$lib/tasks/client';
import { buildLocalResults, buildTaskResults, normalizePaletteQuery } from './model';

describe('command palette result model', () => {
	it('normalizes case, compatibility characters, and extra whitespace', () => {
		expect(normalizePaletteQuery('  ＷＯＲＫ\n  Today  ')).toBe('work today');
	});

	it('filters the shared registry and projects into one stable flat order', () => {
		const projects = [
			testProject({ id: 'work', name: 'Work' }),
			testProject({ id: 'home', name: 'Home' })
		];
		const results = buildLocalResults('  WORK  ', commandRegistry, projects, 'work');

		expect(results.map((result) => result.id)).toEqual([
			'command:project-settings',
			'command:manage-projects',
			'project:work'
		]);
		expect(results.at(-1)).toMatchObject({ kind: 'project', active: true });
	});

	it('consistently hides project commands without an active project', () => {
		const results = buildLocalResults('', commandRegistry, [testProject()], undefined);
		expect(results.some((result) => result.id === 'command:quick-add')).toBe(false);
		expect(results.some((result) => result.id === 'command:manage-projects')).toBe(true);
	});

	it('adds explicit task status, location, and due context', () => {
		const results = buildTaskResults(
			[
				testTask({ sectionId: null }),
				testTask({ id: 'done', status: 'completed', sectionId: 'planning', dueDate: '2026-07-22' })
			],
			[testSection({ id: 'planning', name: 'Planning' })]
		);
		expect(results.map((result) => result.description)).toEqual([
			'Active · Inbox',
			'Completed · Planning · Due 2026-07-22'
		]);
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
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testSection(overrides: Partial<ProjectSection> = {}): ProjectSection {
	return {
		id: 'section-id',
		projectId: 'project-id',
		name: 'Section',
		position: 1024,
		version: 1,
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}

function testTask(overrides: Partial<Task> = {}): Task {
	return {
		id: 'task-id',
		projectId: 'project-id',
		sectionId: null,
		parentId: null,
		title: 'Task',
		description: null,
		status: 'active',
		priority: 0,
		dueDate: null,
		dueTime: null,
		dueTimezone: null,
		position: 1024,
		version: 1,
		completedAt: null,
		createdAt: '2026-07-16T10:00:00Z',
		updatedAt: '2026-07-16T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}
