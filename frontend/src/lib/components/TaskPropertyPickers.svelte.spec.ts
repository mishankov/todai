import { page, userEvent } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Project, ProjectSection } from '$lib/projects/client';
import TaskPropertyPickersHarness from './TaskPropertyPickersHarness.svelte';

describe('TaskPropertyPickers', () => {
	beforeEach(() => localStorage.clear());

	it('selects date, time, and priority presets into the local draft', async () => {
		renderPickers();

		await page.getByRole('button', { name: 'Due date: No date' }).click();
		const tomorrow = page.getByRole('option', { name: /^Tomorrow/ });
		await expect.element(tomorrow).toHaveTextContent(/Tomorrow.+/);
		await tomorrow.click();
		await page.getByRole('button', { name: 'Due time: + Time' }).click();
		await page.getByRole('option', { name: /^Morning/ }).click();
		await page.getByRole('button', { name: 'Priority: None' }).click();
		await page.getByRole('option', { name: 'High' }).click();

		const draft = await readDraft();
		expect(draft.priority).toBe(3);
		expect(draft.dueDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		expect(draft.dueTime).toBe('09:00');
		expect(draft.dueTimezone).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone);
	});

	it('clears date with time and timezone, but clears time without changing date', async () => {
		renderPickers({
			initialDueDate: '2026-07-22',
			initialDueTime: '13:00',
			initialDueTimezone: 'Europe/Moscow'
		});

		await page.getByRole('button', { name: /^Due time:/ }).click();
		await page.getByRole('option', { name: 'No time' }).click();
		let draft = await readDraft();
		expect(draft.dueDate).toBe('2026-07-22');
		expect(draft.dueTime).toBeNull();
		expect(draft.dueTimezone).toBeNull();

		await page.getByRole('button', { name: /^Due date:/ }).click();
		await page.getByRole('option', { name: 'No date' }).click();
		draft = await readDraft();
		expect(draft.dueDate).toBeNull();
		expect(draft.dueTime).toBeNull();
		expect(draft.dueTimezone).toBeNull();
	});

	it('searches projects and dependent sections, resets an incompatible section, and remembers IDs', async () => {
		const projects = ['Home', 'Work', 'Study', 'Travel', 'Archive'].map((name, index) =>
			testProject({ id: `project-${index + 1}`, name })
		);
		const workSection = testSection({
			id: 'work-planning',
			projectId: projects[1].id,
			name: 'Planning'
		});
		const loadSections = vi.fn(async (projectId: string) =>
			projectId === projects[1].id ? [workSection] : []
		);
		renderPickers({ projects, loadSections, initialSectionId: 'old-section' });

		await page.getByRole('button', { name: /^Location:/ }).click();
		await page.getByRole('button', { name: /^Project:/ }).click();
		await page.getByRole('searchbox', { name: 'Search projects' }).fill('work');
		await page.getByRole('option', { name: 'Work' }).click();
		await expect.poll(() => loadSections).toHaveBeenCalledWith(projects[1].id);
		let draft = await readDraft();
		expect(draft.projectId).toBe(projects[1].id);
		expect(draft.sectionId).toBeNull();

		await page.getByRole('button', { name: /^Section:/ }).click();
		await page.getByRole('searchbox', { name: 'Search sections' }).fill('plan');
		await page.getByRole('option', { name: 'Planning' }).click();
		draft = await readDraft();
		expect(draft.sectionId).toBe(workSection.id);
		expect(JSON.parse(localStorage.getItem('todai.quick-picks.recent-projects') ?? '[]')).toEqual([
			projects[1].id
		]);
		expect(
			JSON.parse(
				localStorage.getItem(`todai.quick-picks.recent-sections.${projects[1].id}`) ?? '[]'
			)
		).toEqual([workSection.id]);
	});

	it('shows available current and recent entities first and prunes stale IDs', async () => {
		const projects = ['Home', 'Work', 'Study', 'Travel', 'Archive'].map((name, index) =>
			testProject({ id: `project-${index + 1}`, name })
		);
		const sections = ['Inbox Zero', 'Planning', 'Research', 'Trips', 'Someday'].map((name, index) =>
			testSection({ id: `section-${index + 1}`, projectId: projects[0].id, name })
		);
		localStorage.setItem(
			'todai.quick-picks.recent-projects',
			JSON.stringify([projects[4].id, 'missing-project'])
		);
		localStorage.setItem(
			`todai.quick-picks.recent-sections.${projects[0].id}`,
			JSON.stringify([sections[4].id, 'missing-section'])
		);
		renderPickers({ projects, sections, initialSectionId: sections[0].id });

		await page.getByRole('button', { name: /^Location:/ }).click();
		await page.getByRole('button', { name: 'Project: Home' }).click();
		let labels = Array.from(document.querySelectorAll('[role="option"] strong')).map(
			(element) => element.textContent
		);
		expect(labels.slice(0, 2)).toEqual(['Home', 'Archive']);
		await userEvent.keyboard('{Escape}');

		await page.getByRole('button', { name: 'Section: Inbox Zero' }).click();
		labels = Array.from(document.querySelectorAll('[role="option"] strong')).map(
			(element) => element.textContent
		);
		expect(labels.slice(0, 3)).toEqual(['No section (Inbox)', 'Inbox Zero', 'Someday']);
		expect(JSON.parse(localStorage.getItem('todai.quick-picks.recent-projects') ?? '[]')).toEqual([
			projects[4].id
		]);
		expect(
			JSON.parse(
				localStorage.getItem(`todai.quick-picks.recent-sections.${projects[0].id}`) ?? '[]'
			)
		).toEqual([sections[4].id]);
	});

	it('supports arrow selection and restores focus to the trigger on Escape', async () => {
		renderPickers();
		const trigger = page.getByRole('button', { name: 'Due date: No date' });
		await trigger.click();
		await userEvent.keyboard('{ArrowUp}{Enter}');
		let draft = await readDraft();
		expect(draft.dueDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);

		await page.getByRole('button', { name: /^Due date:/ }).click();
		await userEvent.keyboard('{Escape}');
		await expect.element(page.getByRole('button', { name: /^Due date:/ })).toHaveFocus();
		draft = await readDraft();
		expect(draft.dueDate).not.toBeNull();
	});
});

interface PickerOptions {
	projects?: Project[];
	sections?: ProjectSection[];
	loadSections?: (projectId: string) => Promise<ProjectSection[]>;
	initialSectionId?: string | null;
	initialDueDate?: string | null;
	initialDueTime?: string | null;
	initialDueTimezone?: string | null;
}

function renderPickers(options: PickerOptions = {}) {
	const projects = options.projects ?? [testProject()];
	return render(TaskPropertyPickersHarness, {
		projects,
		sections: options.sections ?? [],
		loadSections: options.loadSections,
		initialProjectId: projects[0].id,
		initialSectionId: options.initialSectionId,
		initialDueDate: options.initialDueDate,
		initialDueTime: options.initialDueTime,
		initialDueTimezone: options.initialDueTimezone
	});
}

function readDraft() {
	const text = document.querySelector('[data-testid="property-draft"]')?.textContent ?? '{}';
	return JSON.parse(text) as {
		projectId: string;
		sectionId: string | null;
		priority: number;
		dueDate: string | null;
		dueTime: string | null;
		dueTimezone: string | null;
	};
}

function testProject(overrides: Partial<Project> = {}): Project {
	return {
		id: 'project-id',
		name: 'Project',
		layout: 'list',
		colorTheme: 'sage',
		agentModel: 'model',
		agentThinkingEffort: 'medium',
		position: 1024,
		version: 1,
		archivedAt: null,
		createdAt: '',
		updatedAt: '',
		lastModifiedBy: '',
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
		createdAt: '',
		updatedAt: '',
		lastModifiedBy: '',
		...overrides
	};
}
