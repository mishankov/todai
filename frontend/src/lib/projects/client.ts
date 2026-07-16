export interface Project {
	id: string;
	name: string;
	position: number;
	version: number;
	archivedAt: string | null;
	createdAt: string;
	updatedAt: string;
	lastModifiedBy: string;
}

export interface ProjectUpdate {
	version: number;
	name?: string;
	archived?: boolean;
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
