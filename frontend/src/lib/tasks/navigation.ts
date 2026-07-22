/* Task destinations are assembled from persisted task identities and validated internal paths. */
/* eslint-disable svelte/no-navigation-without-resolve */
import { browser } from '$app/environment';
import { goto, pushState, replaceState } from '$app/navigation';
import { page } from '$app/state';
import type { Task } from './client';

export interface TaskRoute {
	projectId: string;
	taskId: string;
}

export const taskNavigationEvent = 'todai:task-navigation';

const authenticatedPaths = [
	/^\/$/,
	/^\/projects$/,
	/^\/settings$/,
	/^\/activity$/,
	/^\/today$/,
	/^\/projects\/[^/]+(?:\/(?:overview|today|tasks|activity|settings))?$/
];

export function canonicalTaskPath(projectId: string, taskId: string): string {
	return `/projects/${encodeURIComponent(projectId)}/tasks/${encodeURIComponent(taskId)}`;
}

export function canonicalTaskUrl(task: Pick<Task, 'projectId' | 'id'>, origin: string): string {
	return new URL(canonicalTaskPath(task.projectId, task.id), origin).href;
}

export function parseTaskPath(pathname: string): TaskRoute | null {
	const match = pathname.match(/^\/projects\/([^/]+)\/tasks\/([^/]+)\/?$/);
	if (!match) return null;
	try {
		return { projectId: decodeURIComponent(match[1]), taskId: decodeURIComponent(match[2]) };
	} catch {
		return null;
	}
}

export function canonicalProjectMismatch(task: Pick<Task, 'projectId' | 'id'>, route: TaskRoute) {
	return task.projectId === route.projectId ? null : canonicalTaskPath(task.projectId, task.id);
}

export function defaultTaskReturnPath(projectId: string): string {
	return `/projects/${encodeURIComponent(projectId)}/tasks`;
}

export function validTaskReturnLocation(value: unknown): value is string {
	if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) return false;
	let url: URL;
	try {
		url = new URL(value, 'https://todai.local');
	} catch {
		return false;
	}
	return (
		url.origin === 'https://todai.local' &&
		parseTaskPath(url.pathname) === null &&
		authenticatedPaths.some((pattern) => pattern.test(url.pathname))
	);
}

export function validPostLoginRedirect(value: unknown): value is string {
	if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) return false;
	let url: URL;
	try {
		url = new URL(value, 'https://todai.local');
	} catch {
		return false;
	}
	return (
		url.origin === 'https://todai.local' &&
		(parseTaskPath(url.pathname) !== null ||
			authenticatedPaths.some((pattern) => pattern.test(url.pathname)))
	);
}

export function openTask(task: Task): void {
	if (!browser) return;
	const currentRoute = parseTaskPath(window.location.pathname);
	const existingReturn = page.state.taskModal?.returnTo;
	const returnTo = validTaskReturnLocation(existingReturn)
		? existingReturn
		: validTaskReturnLocation(currentLocation())
			? currentLocation()
			: defaultTaskReturnPath(task.projectId);
	const state: App.PageState = { ...page.state, taskModal: { returnTo } };
	const path = canonicalTaskPath(task.projectId, task.id);
	if (currentRoute) replaceState(path, state);
	else pushState(path, state);
	window.dispatchEvent(new CustomEvent(taskNavigationEvent));
}

export function closeTask(route: TaskRoute): void {
	if (!browser) return;
	const returnTo = page.state.taskModal?.returnTo;
	if (validTaskReturnLocation(returnTo)) {
		const currentPath = window.location.href;
		window.history.back();
		window.setTimeout(() => {
			if (window.location.href !== currentPath) return;
			void goto(returnTo, {
				replaceState: true,
				keepFocus: true,
				noScroll: true,
				state: withoutTaskModal(page.state)
			});
		}, 150);
		return;
	}
	void goto(defaultTaskReturnPath(route.projectId), {
		replaceState: true,
		keepFocus: true,
		noScroll: true,
		state: withoutTaskModal(page.state)
	});
}

export async function replaceMismatchedTaskRoute(task: Task, route: TaskRoute): Promise<boolean> {
	const canonical = canonicalProjectMismatch(task, route);
	if (!canonical) return false;
	replaceState(canonical, {
		...page.state,
		taskModal: { ...page.state.taskModal }
	});
	window.dispatchEvent(new CustomEvent(taskNavigationEvent));
	return true;
}

function currentLocation(): string {
	return `${window.location.pathname}${window.location.search}${window.location.hash}`;
}

function withoutTaskModal(state: App.PageState): App.PageState {
	const rest = { ...state };
	delete rest.taskModal;
	return rest;
}
