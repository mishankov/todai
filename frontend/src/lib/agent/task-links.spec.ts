import { describe, expect, it } from 'vitest';
import { parseAgentTaskLinks } from './task-links';

describe('assistant task links', () => {
	it('extracts canonical task deep links while preserving surrounding text', () => {
		expect(
			parseAgentTaskLinks(
				'Start with [Plan tomorrow](/projects/project-id/tasks/task-id), then rest.'
			)
		).toEqual([
			{ text: 'Start with ' },
			{
				text: 'Plan tomorrow',
				href: '/projects/project-id/tasks/task-id'
			},
			{ text: ', then rest.' }
		]);
	});

	it('supports encoded canonical task identities', () => {
		expect(parseAgentTaskLinks('[Open task](/projects/project%20one/tasks/task%20two)')).toEqual([
			{
				text: 'Open task',
				href: '/projects/project%20one/tasks/task%20two'
			}
		]);
	});

	it('leaves external, malformed, and non-canonical links as plain text', () => {
		for (const content of [
			'[External](https://example.com/task)',
			'[Wrong route](/tasks/task-id)',
			'[Query](/projects/project-id/tasks/task-id?from=chat)',
			'[Trailing slash](/projects/project-id/tasks/task-id/)'
		]) {
			expect(parseAgentTaskLinks(content)).toEqual([{ text: content }]);
		}
	});
});
