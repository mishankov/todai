import { browser } from '$app/environment';
import type { Project } from './client';

const lastProjectKey = 'todai.last-project-id';
const projectViewSuffixes = [
	'',
	'/overview',
	'/today',
	'/tasks',
	'/activity',
	'/settings'
] as const;

export function initialProjectPath(projects: Project[], suffix = ''): string {
	const rememberedId = browser ? window.localStorage.getItem(lastProjectKey) : null;
	const project = projects.find((candidate) => candidate.id === rememberedId) ?? projects[0];
	return project ? `/projects/${project.id}${suffix}` : '/projects';
}

export function rememberedProjectPath(projectId: string): string {
	const prefix = `/projects/${projectId}`;
	const remembered = browser ? window.localStorage.getItem(lastViewKey(projectId)) : null;
	const suffix = remembered?.startsWith(prefix) ? remembered.slice(prefix.length) : '/overview';
	return `${prefix}${isProjectViewSuffix(suffix) ? suffix : '/overview'}`;
}

export function recordProjectPath(projectId: string, path: string): void {
	if (!browser || !path.startsWith(`/projects/${projectId}`)) return;
	window.localStorage.setItem(lastProjectKey, projectId);
	window.localStorage.setItem(lastViewKey(projectId), path);
}

function isProjectViewSuffix(value: string): value is (typeof projectViewSuffixes)[number] {
	return projectViewSuffixes.includes(value as (typeof projectViewSuffixes)[number]);
}

function lastViewKey(projectId: string): string {
	return `todai.project.${projectId}.last-view`;
}
