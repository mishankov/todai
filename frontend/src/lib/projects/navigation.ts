import { browser } from '$app/environment';
import type { Project } from './client';

const lastProjectKey = 'todai.last-project-id';
const projectContextParameter = 'project';
const projectViewSuffixes = [
	'',
	'/overview',
	'/today',
	'/tasks',
	'/activity',
	'/settings'
] as const;

export type AccountDestination = '/projects' | '/settings';

export function accountDestinationPath(
	destination: AccountDestination,
	projectId?: string
): string {
	if (!projectId) return destination;
	const parameters = new URLSearchParams({ [projectContextParameter]: projectId });
	return `${destination}?${parameters}`;
}

export function activeProjectFromLocation(
	projects: readonly Project[],
	pathname: string,
	search = ''
): Project | undefined {
	const projectId = projectIdFromLocation(pathname, search);
	return projectId
		? projects.find((project) => project.id === projectId && project.archivedAt === null)
		: undefined;
}

export function projectIdFromLocation(pathname: string, search = ''): string | undefined {
	const match = pathname.match(/^\/projects\/([^/]+)/);
	if (match) {
		try {
			return decodeURIComponent(match[1]);
		} catch {
			return undefined;
		}
	}
	if (pathname !== '/projects' && pathname !== '/settings') return undefined;
	return new URLSearchParams(search).get(projectContextParameter) || undefined;
}

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
	const prefix = `/projects/${projectId}`;
	if (!browser || !path.startsWith(prefix)) return;
	const suffix = path.slice(prefix.length);
	if (!isProjectViewSuffix(suffix)) return;
	window.localStorage.setItem(lastProjectKey, projectId);
	window.localStorage.setItem(lastViewKey(projectId), path);
}

function isProjectViewSuffix(value: string): value is (typeof projectViewSuffixes)[number] {
	return projectViewSuffixes.includes(value as (typeof projectViewSuffixes)[number]);
}

function lastViewKey(projectId: string): string {
	return `todai.project.${projectId}.last-view`;
}
