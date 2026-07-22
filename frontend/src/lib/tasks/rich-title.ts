export type TitleProperty = 'project' | 'section' | 'priority' | 'due';

export interface ActiveTitleToken {
	type: TitleProperty;
	trigger: '#' | '/' | '!' | '@';
	start: number;
	end: number;
	query: string;
}

export interface TitleOption {
	id: string;
	label: string;
	detail?: string;
	custom?: 'date' | 'time';
}

const triggerTypes: Record<ActiveTitleToken['trigger'], TitleProperty> = {
	'#': 'project',
	'/': 'section',
	'!': 'priority',
	'@': 'due'
};

export function detectTitleToken(
	value: string,
	caret: number,
	dismissedStart: number | null = null
): ActiveTitleToken | null {
	if (caret < 1 || caret > value.length) return null;
	let start = caret - 1;
	while (start > 0 && !/\s/u.test(value[start - 1])) start -= 1;
	const trigger = value[start] as ActiveTitleToken['trigger'];
	if (!(trigger in triggerTypes) || start === dismissedStart) return null;
	if (start > 0 && !/\s/u.test(value[start - 1])) return null;

	let end = caret;
	while (end < value.length && !/\s/u.test(value[end])) end += 1;
	return {
		type: triggerTypes[trigger],
		trigger,
		start,
		end,
		query: value.slice(start + 1, caret)
	};
}

export function filterTitleOptions<T extends TitleOption>(options: T[], query: string): T[] {
	const normalized = query.trim().toLocaleLowerCase();
	if (!normalized) return options;
	return options.filter((option) =>
		`${option.label} ${option.detail ?? ''}`.toLocaleLowerCase().includes(normalized)
	);
}

export function removeTitleToken(
	value: string,
	token: Pick<ActiveTitleToken, 'start' | 'end'>
): { value: string; caret: number } {
	const before = value.slice(0, token.start).replace(/\s+$/u, '');
	const after = value.slice(token.end).replace(/^\s+/u, '');
	const separator = before && after ? ' ' : '';
	return { value: `${before}${separator}${after}`, caret: before.length + separator.length };
}

export function cleanTaskTitle(value: string): string {
	return value.trim().replace(/\s+/gu, ' ');
}
