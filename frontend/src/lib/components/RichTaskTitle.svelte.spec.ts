import type { Project, ProjectSection } from '$lib/projects/client';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import RichTaskTitleHarness from './RichTaskTitleHarness.svelte';

describe('RichTaskTitle', () => {
	it('hides preset location chips until the user explicitly selects them', async () => {
		const project = testProject({ id: 'work', name: 'Work' });
		const section = testSection({ id: 'planning', projectId: project.id, name: 'Planning' });
		render(RichTaskTitleHarness, {
			projects: [project],
			sections: [section],
			initialProjectId: project.id,
			initialSectionId: section.id,
			hidePresetLocationChips: true
		});

		await expect
			.element(page.getByRole('button', { name: 'project: Work. Open picker' }))
			.not.toBeInTheDocument();
		await expect
			.element(page.getByRole('button', { name: 'section: Planning. Open picker' }))
			.not.toBeInTheDocument();
		expect(readDraft()).toMatchObject({ projectId: project.id, sectionId: section.id });

		const input = page.getByRole('combobox', { name: 'Task title' });
		await input.fill('#work');
		await userEvent.keyboard('{Enter}');
		await expect
			.element(page.getByRole('button', { name: 'project: Work. Open picker' }))
			.toBeVisible();
		await userEvent.keyboard(' /plan');
		await userEvent.keyboard('{Enter}');
		await expect
			.element(page.getByRole('button', { name: 'section: Planning. Open picker' }))
			.toBeVisible();
	});

	it('selects every property with the keyboard and leaves only a clean title', async () => {
		const home = testProject({ id: 'home', name: 'Home' });
		const work = testProject({ id: 'work', name: 'Work' });
		const planning = testSection({ id: 'planning', projectId: work.id, name: 'Planning' });
		const loadSections = vi.fn(async (projectId: string) =>
			projectId === work.id ? [planning] : []
		);
		render(RichTaskTitleHarness, { projects: [home, work], loadSections });
		const input = page.getByRole('combobox', { name: 'Task title' });

		await input.fill('Prepare report #wo');
		await userEvent.keyboard('{Enter}');
		await expect.element(input).toHaveValue('Prepare report');
		await expect.poll(() => loadSections).toHaveBeenCalledWith('work');

		await userEvent.keyboard(' /pla');
		await userEvent.keyboard('{Enter}');
		await userEvent.keyboard(' !hi');
		await userEvent.keyboard('{Enter}');
		await userEvent.keyboard(' @tom');
		await userEvent.keyboard('{Enter}');
		await expect.element(page.getByRole('option', { name: /^Morning/ })).toBeVisible();
		await userEvent.keyboard('{Enter}');

		const draft = readDraft();
		expect(draft.title).toBe('Prepare report');
		expect(draft.projectId).toBe('work');
		expect(draft.sectionId).toBe('planning');
		expect(draft.priority).toBe(3);
		expect(draft.dueDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		expect(draft.dueTime).toBe('09:00');
		expect(draft.dueTimezone).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone);
		await expect
			.element(page.getByRole('button', { name: 'priority: High. Open picker' }))
			.toBeVisible();
		await expect.element(input).toHaveFocus();
	});

	it('keeps a dismissed trigger literal until the caret moves to another token', async () => {
		render(RichTaskTitleHarness, { projects: [testProject()] });
		const input = page.getByRole('combobox', { name: 'Task title' });
		await input.fill('Use C# and #literal');
		await expect.element(page.getByRole('listbox')).toBeVisible();
		await userEvent.keyboard('{Escape}{ArrowLeft}{ArrowRight}');
		await expect.element(page.getByRole('listbox')).not.toBeInTheDocument();
		expect(readDraft().title).toBe('Use C# and #literal');

		await userEvent.keyboard(' #pro');
		await expect.element(page.getByRole('listbox')).toBeVisible();
	});

	it('resets an incompatible section with an announced message when project changes', async () => {
		const home = testProject({ id: 'home', name: 'Home' });
		const work = testProject({ id: 'work', name: 'Work' });
		const homeSection = testSection({ id: 'chores', projectId: home.id, name: 'Chores' });
		render(RichTaskTitleHarness, {
			projects: [home, work],
			sections: [homeSection],
			initialTitle: 'Task ',
			initialProjectId: home.id,
			initialSectionId: homeSection.id
		});
		const input = page.getByRole('combobox', { name: 'Task title' });
		await input.fill('Task #work');
		await page.getByRole('option', { name: 'Work' }).click();

		expect(readDraft().sectionId).toBeNull();
		await expect
			.element(page.getByText('The previous section was removed because the project changed.'))
			.toBeInTheDocument();
	});

	it('stays synchronized with ordinary property controls and supports chip deletion', async () => {
		render(RichTaskTitleHarness, { projects: [testProject()], showControls: true });
		await page.getByRole('button', { name: 'Priority: None' }).click();
		await page.getByRole('option', { name: 'High' }).click();
		const chip = page.getByRole('button', { name: 'priority: High. Open picker' });
		await expect.element(chip).toBeVisible();
		document
			.querySelector<HTMLButtonElement>('[aria-label="priority: High. Open picker"]')!
			.focus();
		await userEvent.keyboard('{Delete}');
		expect(readDraft().priority).toBe(0);
	});

	it('does not select an option while an IME composition is active', async () => {
		render(RichTaskTitleHarness, {
			projects: [testProject({ id: 'work', name: 'Work' })],
			initialProjectId: ''
		});
		const input = page.getByRole('combobox', { name: 'Task title' });
		await input.fill('#');
		const element = document.querySelector<HTMLInputElement>('.title-input')!;
		element.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
		await userEvent.keyboard('{Enter}');
		expect(readDraft().projectId).toBe('');
		element.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true }));
		await userEvent.keyboard('{Enter}');
		expect(readDraft().projectId).toBe('work');
	});
});

function readDraft() {
	const text = document.querySelector('[data-testid="rich-title-draft"]')?.textContent ?? '{}';
	return JSON.parse(text) as {
		title: string;
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
