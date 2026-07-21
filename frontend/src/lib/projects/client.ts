export interface Project {
	id: string;
	name: string;
	layout: ProjectLayout;
	colorTheme: ProjectColorTheme;
	agentModel: string;
	agentThinkingEffort: AgentThinkingEffort;
	position: number;
	version: number;
	archivedAt: string | null;
	createdAt: string;
	updatedAt: string;
	lastModifiedBy: string;
}

export type ProjectLayout = 'list' | 'board';
export type ProjectColorTheme = 'sage' | 'ocean' | 'plum' | 'sand' | 'rose' | 'graphite';
export type AgentThinkingEffort = 'off' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max';

export const projectColorThemes: ReadonlyArray<{
	id: ProjectColorTheme;
	name: string;
	description: string;
}> = [
	{ id: 'sage', name: 'Sage', description: 'Calm green' },
	{ id: 'ocean', name: 'Ocean', description: 'Clear blue' },
	{ id: 'plum', name: 'Plum', description: 'Deep violet' },
	{ id: 'sand', name: 'Sand', description: 'Warm neutral' },
	{ id: 'rose', name: 'Rose', description: 'Soft red' },
	{ id: 'graphite', name: 'Graphite', description: 'Quiet monochrome' }
];

export interface ProjectSection {
	id: string;
	projectId: string;
	name: string;
	position: number;
	version: number;
	createdAt: string;
	updatedAt: string;
	lastModifiedBy: string;
}

export interface ProjectUpdate {
	version: number;
	name?: string;
	archived?: boolean;
	layout?: ProjectLayout;
	colorTheme?: ProjectColorTheme;
	agentModel?: string;
	agentThinkingEffort?: AgentThinkingEffort;
}

export class ProjectRequestError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'ProjectRequestError';
	}
}

export class ProjectConflictError extends ProjectRequestError {
	constructor() {
		super('The project changed after it was opened.');
		this.name = 'ProjectConflictError';
	}
}

export async function listProjects(
	fetcher: typeof fetch,
	includeArchived = false
): Promise<Project[]> {
	const query = new URLSearchParams({ include_archived: String(includeArchived) });
	const response = await fetcher(`/api/projects?${query}`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) throw new ProjectRequestError('Could not load projects.');

	const body = (await response.json()) as { projects: Project[] };
	return body.projects;
}

export async function getProject(fetcher: typeof fetch, projectId: string): Promise<Project> {
	const response = await fetcher(`/api/projects/${encodeURIComponent(projectId)}`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) throw new ProjectRequestError('Could not load the project.');
	return (await response.json()) as Project;
}

export async function createProject(fetcher: typeof fetch, name: string): Promise<Project> {
	const response = await fetcher('/api/projects', {
		method: 'POST',
		credentials: 'same-origin',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify({ name })
	});
	if (!response.ok) throw new ProjectRequestError('Could not create the project.');
	return (await response.json()) as Project;
}

export async function updateProject(
	fetcher: typeof fetch,
	projectId: string,
	update: ProjectUpdate
): Promise<Project> {
	const response = await fetcher(`/api/projects/${encodeURIComponent(projectId)}`, {
		method: 'PATCH',
		credentials: 'same-origin',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify(update)
	});
	if (response.status === 409) throw new ProjectConflictError();
	if (!response.ok) throw new ProjectRequestError('Could not update the project.');
	return (await response.json()) as Project;
}

export async function listProjectSections(
	fetcher: typeof fetch,
	projectId: string
): Promise<ProjectSection[]> {
	const response = await fetcher(`/api/projects/${encodeURIComponent(projectId)}/sections`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) throw new ProjectRequestError('Could not load project sections.');
	const body = (await response.json()) as { sections: ProjectSection[] };
	return body.sections;
}

export async function createProjectSection(
	fetcher: typeof fetch,
	projectId: string,
	name: string
): Promise<ProjectSection> {
	const response = await fetcher(`/api/projects/${encodeURIComponent(projectId)}/sections`, {
		method: 'POST',
		credentials: 'same-origin',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify({ name })
	});
	if (!response.ok) throw new ProjectRequestError('Could not create the project section.');
	return (await response.json()) as ProjectSection;
}

export async function updateProjectSection(
	fetcher: typeof fetch,
	projectId: string,
	sectionId: string,
	version: number,
	name: string
): Promise<ProjectSection> {
	const response = await fetcher(
		`/api/projects/${encodeURIComponent(projectId)}/sections/${encodeURIComponent(sectionId)}`,
		{
			method: 'PATCH',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({ version, name })
		}
	);
	if (response.status === 409) throw new ProjectConflictError();
	if (!response.ok) throw new ProjectRequestError('Could not update the project section.');
	return (await response.json()) as ProjectSection;
}

export async function deleteProjectSection(
	fetcher: typeof fetch,
	projectId: string,
	sectionId: string,
	version: number
): Promise<void> {
	const response = await fetcher(
		`/api/projects/${encodeURIComponent(projectId)}/sections/${encodeURIComponent(sectionId)}`,
		{
			method: 'DELETE',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({ version })
		}
	);
	if (response.status === 409) throw new ProjectConflictError();
	if (!response.ok) throw new ProjectRequestError('Could not delete the project section.');
}

export async function reorderProjectSection(
	fetcher: typeof fetch,
	projectId: string,
	sectionId: string,
	version: number,
	beforeSectionId: string | null
): Promise<ProjectSection[]> {
	const response = await fetcher(
		`/api/projects/${encodeURIComponent(projectId)}/sections/${encodeURIComponent(sectionId)}/reorder`,
		{
			method: 'POST',
			credentials: 'same-origin',
			headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: JSON.stringify({ version, beforeSectionId })
		}
	);
	if (response.status === 409) throw new ProjectConflictError();
	if (!response.ok) throw new ProjectRequestError('Could not reorder project sections.');
	const body = (await response.json()) as { sections: ProjectSection[] };
	return body.sections;
}
