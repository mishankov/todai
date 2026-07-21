import { browser } from '$app/environment';
import type { Project } from './client';

const lastProjectKey = 'todai.last-project-id';

export function initialProjectPath(projects: Project[], suffix = ''): string {
	const rememberedId = browser ? window.localStorage.getItem(lastProjectKey) : null;
	const project = projects.find((candidate) => candidate.id === rememberedId) ?? projects[0];
	return project ? `/projects/${project.id}${suffix}` : '/projects';
}
