import type { Project, ProjectSection } from '$lib/projects/client';

export interface QuickPickItem {
	id: string;
	label: string;
	detail?: string;
	group?: string;
	custom?: boolean;
}

export interface TaskPropertyDraft {
	projectId: string;
	sectionId: string | null;
	priority: number;
	dueDate: string | null;
	dueTime: string | null;
	dueTimezone: string | null;
}

export interface RelativeDateOption {
	id: 'today' | 'tomorrow' | 'next-week';
	label: string;
	value: string;
	date: Date;
}

export const priorityOptions = [
	{ value: 0, label: 'None' },
	{ value: 1, label: 'Low' },
	{ value: 2, label: 'Medium' },
	{ value: 3, label: 'High' },
	{ value: 4, label: 'Urgent' }
] as const;

export const timeOptions = [
	{ value: '09:00', label: 'Morning' },
	{ value: '13:00', label: 'Afternoon' },
	{ value: '18:00', label: 'Evening' }
] as const;

export function relativeDateOptions(now = new Date()): RelativeDateOption[] {
	const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
	const tomorrow = addLocalDays(today, 1);
	const daysUntilNextWeek = today.getDay() === 0 ? 1 : 8 - today.getDay();
	const nextWeek = addLocalDays(today, daysUntilNextWeek);

	return [
		{ id: 'today', label: 'Today', value: localDateValue(today), date: today },
		{ id: 'tomorrow', label: 'Tomorrow', value: localDateValue(tomorrow), date: tomorrow },
		{
			id: 'next-week',
			label: 'Next week',
			value: localDateValue(nextWeek),
			date: nextWeek
		}
	];
}

export function addLocalDays(value: Date, days: number): Date {
	return new Date(value.getFullYear(), value.getMonth(), value.getDate() + days);
}

export function localDateValue(value: Date): string {
	const year = value.getFullYear();
	const month = String(value.getMonth() + 1).padStart(2, '0');
	const day = String(value.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

export function parseLocalDate(value: string): Date {
	const [year, month, day] = value.split('-').map(Number);
	return new Date(year, month - 1, day);
}

export function formatDate(value: string | Date): string {
	const date = typeof value === 'string' ? parseLocalDate(value) : value;
	return new Intl.DateTimeFormat(undefined, {
		month: 'short',
		day: 'numeric',
		...(date.getFullYear() === new Date().getFullYear() ? {} : { year: 'numeric' })
	}).format(date);
}

export function formatTime(value: string): string {
	const [hours, minutes] = value.split(':').map(Number);
	return new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(
		new Date(2000, 0, 1, hours, minutes)
	);
}

export function entityItems<T extends Project | ProjectSection>(
	entities: T[],
	currentId: string,
	recentIds: string[]
): QuickPickItem[] {
	const available = new Map(entities.map((entity) => [entity.id, entity]));
	const quickIds = [currentId, ...recentIds]
		.filter((id, index, ids) => id && ids.indexOf(id) === index && available.has(id))
		.slice(0, 4);
	const useGroups = entities.length > 4 && quickIds.length > 0;
	if (!useGroups) return entities.map((entity) => ({ id: entity.id, label: entity.name }));

	return [
		...quickIds.map((id) => ({
			id,
			label: available.get(id)!.name,
			group: id === currentId ? 'Current' : 'Recent'
		})),
		...entities
			.filter((entity) => !quickIds.includes(entity.id))
			.map((entity) => ({ id: entity.id, label: entity.name, group: 'All' }))
	];
}

export function readRecentIds(storage: Storage, key: string, availableIds: string[]): string[] {
	let stored: unknown;
	try {
		stored = JSON.parse(storage.getItem(key) ?? '[]');
	} catch {
		stored = [];
	}
	const available = new Set(availableIds);
	const result = Array.isArray(stored)
		? stored.filter((value): value is string => typeof value === 'string' && available.has(value))
		: [];
	storage.setItem(key, JSON.stringify(result.slice(0, 3)));
	return result.slice(0, 3);
}

export function rememberRecentId(storage: Storage, key: string, id: string): string[] {
	let stored: unknown;
	try {
		stored = JSON.parse(storage.getItem(key) ?? '[]');
	} catch {
		stored = [];
	}
	const current = Array.isArray(stored)
		? stored.filter((value): value is string => typeof value === 'string')
		: [];
	const result = [id, ...current.filter((value) => value !== id)].slice(0, 3);
	storage.setItem(key, JSON.stringify(result));
	return result;
}
