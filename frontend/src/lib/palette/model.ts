import type { Project, ProjectSection } from '$lib/projects/client';
import type { ProductCommand } from '$lib/shortcuts/registry';
import type { Task } from '$lib/tasks/client';

export interface CommandPaletteResult {
	kind: 'command';
	id: string;
	group: 'Commands';
	label: string;
	description: string;
	command: ProductCommand;
}

export interface ProjectPaletteResult {
	kind: 'project';
	id: string;
	group: 'Projects';
	label: string;
	description: string;
	project: Project;
	active: boolean;
}

export interface TaskPaletteResult {
	kind: 'task';
	id: string;
	group: 'Tasks';
	label: string;
	description: string;
	task: Task;
}

export type PaletteResult = CommandPaletteResult | ProjectPaletteResult | TaskPaletteResult;

export function normalizePaletteQuery(value: string): string {
	return value.normalize('NFKC').trim().replace(/\s+/g, ' ').toLocaleLowerCase();
}

export function buildLocalResults(
	query: string,
	commands: readonly ProductCommand[],
	projects: readonly Project[],
	activeProjectId?: string
): PaletteResult[] {
	const normalized = normalizePaletteQuery(query);
	const commandResults: CommandPaletteResult[] = commands
		.filter((command) => command.id !== 'command-palette')
		.filter((command) => command.scope === 'global' || activeProjectId !== undefined)
		.filter((command) =>
			matchesPaletteQuery(
				[command.label, command.description, ...command.aliases].join(' '),
				normalized
			)
		)
		.map((command) => ({
			kind: 'command',
			id: `command:${command.id}`,
			group: 'Commands',
			label: command.label,
			description: command.description,
			command
		}));
	const projectResults: ProjectPaletteResult[] = projects
		.filter((project) => project.archivedAt === null)
		.filter((project) => matchesPaletteQuery(project.name, normalized))
		.map((project) => ({
			kind: 'project',
			id: `project:${project.id}`,
			group: 'Projects',
			label: project.name,
			description: project.id === activeProjectId ? 'Active project' : 'Switch project',
			project,
			active: project.id === activeProjectId
		}));
	return [...commandResults, ...projectResults];
}

export function buildTaskResults(
	tasks: readonly Task[],
	sections: readonly ProjectSection[]
): TaskPaletteResult[] {
	const sectionNames = new Map(sections.map((section) => [section.id, section.name]));
	return tasks.map((task) => {
		const status = task.status === 'completed' ? 'Completed' : 'Active';
		const location = task.sectionId ? (sectionNames.get(task.sectionId) ?? 'Section') : 'Inbox';
		const due = task.dueDate ? ` · Due ${task.dueDate}` : '';
		return {
			kind: 'task',
			id: `task:${task.id}`,
			group: 'Tasks',
			label: task.title,
			description: `${status} · ${location}${due}`,
			task
		};
	});
}

function matchesPaletteQuery(value: string, normalizedQuery: string): boolean {
	return normalizedQuery === '' || normalizePaletteQuery(value).includes(normalizedQuery);
}
