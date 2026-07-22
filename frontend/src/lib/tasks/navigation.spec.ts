import { describe, expect, it } from 'vitest';
import type { Task } from './client';
import {
	canonicalProjectMismatch,
	canonicalTaskPath,
	canonicalTaskUrl,
	defaultTaskReturnPath,
	parseTaskPath,
	validPostLoginRedirect,
	validTaskReturnLocation
} from './navigation';

describe('task deep-link navigation', () => {
	it('builds and parses encoded canonical task paths', () => {
		const path = canonicalTaskPath('project / one', 'task / two');

		expect(path).toBe('/projects/project%20%2F%20one/tasks/task%20%2F%20two');
		expect(parseTaskPath(path)).toEqual({
			projectId: 'project / one',
			taskId: 'task / two'
		});
		expect(canonicalTaskUrl(testTask(), 'https://todai.example/base')).toBe(
			'https://todai.example/projects/project-id/tasks/task-id'
		);
	});

	it('rejects malformed and non-canonical task paths', () => {
		expect(parseTaskPath('/tasks/task-id')).toBeNull();
		expect(parseTaskPath('/projects/project-id/tasks')).toBeNull();
		expect(parseTaskPath('/projects/%E0%A4%A/tasks/task-id')).toBeNull();
	});

	it('detects a stale project id without changing a canonical route', () => {
		const task = testTask({ projectId: 'current-project' });

		expect(canonicalProjectMismatch(task, { projectId: 'old-project', taskId: task.id })).toBe(
			'/projects/current-project/tasks/task-id'
		);
		expect(
			canonicalProjectMismatch(task, { projectId: task.projectId, taskId: task.id })
		).toBeNull();
		expect(defaultTaskReturnPath(task.projectId)).toBe('/projects/current-project/tasks');
	});

	it('accepts only standard authenticated views as modal return locations', () => {
		expect(validTaskReturnLocation('/projects/project-id/today?filter=open#heading')).toBe(true);
		expect(validTaskReturnLocation('/projects/project-id')).toBe(true);
		expect(validTaskReturnLocation('/projects/project-id/tasks/task-id')).toBe(false);
		expect(validTaskReturnLocation('//evil.example/projects/project-id')).toBe(false);
		expect(validTaskReturnLocation('https://evil.example/projects/project-id')).toBe(false);
		expect(validTaskReturnLocation('/login')).toBe(false);
	});

	it('allows internal task links but rejects external post-login redirects', () => {
		expect(validPostLoginRedirect('/projects/project-id/tasks/task-id?from=login')).toBe(true);
		expect(validPostLoginRedirect('/projects/project-id/overview')).toBe(true);
		expect(validPostLoginRedirect('//evil.example/steal-session')).toBe(false);
		expect(validPostLoginRedirect('https://evil.example/steal-session')).toBe(false);
		expect(validPostLoginRedirect('/login')).toBe(false);
	});
});

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
		createdAt: '2026-07-22T10:00:00Z',
		updatedAt: '2026-07-22T10:00:00Z',
		lastModifiedBy: 'user-id',
		...overrides
	};
}
