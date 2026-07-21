export type ShortcutCommandId =
	| 'quick-add'
	| 'toggle-chat'
	| 'project-overview'
	| 'project-inbox'
	| 'project-today'
	| 'project-tasks'
	| 'project-activity'
	| 'project-settings'
	| 'toggle-help';

export interface ShortcutCommand {
	id: ShortcutCommandId;
	label: string;
	description: string;
	code: string;
	keyLabel: string;
	scope: 'global' | 'project';
}

export const shortcutCommands: readonly ShortcutCommand[] = [
	{
		id: 'quick-add',
		label: 'Quick add',
		description: 'Create a task from anywhere in the active project',
		code: 'KeyN',
		keyLabel: 'N',
		scope: 'project'
	},
	{
		id: 'toggle-chat',
		label: 'Assistant',
		description: 'Open or close the active project assistant',
		code: 'KeyJ',
		keyLabel: 'J',
		scope: 'project'
	},
	{
		id: 'project-overview',
		label: 'Project overview',
		description: 'Open the active project overview',
		code: 'Digit1',
		keyLabel: '1',
		scope: 'project'
	},
	{
		id: 'project-inbox',
		label: 'Inbox',
		description: 'Open the active project Inbox',
		code: 'Digit2',
		keyLabel: '2',
		scope: 'project'
	},
	{
		id: 'project-today',
		label: 'Today',
		description: 'Open the active project Today view',
		code: 'Digit3',
		keyLabel: '3',
		scope: 'project'
	},
	{
		id: 'project-tasks',
		label: 'Tasks',
		description: 'Open all tasks in the active project',
		code: 'Digit4',
		keyLabel: '4',
		scope: 'project'
	},
	{
		id: 'project-activity',
		label: 'Activity',
		description: 'Open the active project activity feed',
		code: 'Digit5',
		keyLabel: '5',
		scope: 'project'
	},
	{
		id: 'project-settings',
		label: 'Project settings',
		description: 'Open the active project settings',
		code: 'Digit6',
		keyLabel: '6',
		scope: 'project'
	},
	{
		id: 'toggle-help',
		label: 'Keyboard shortcuts',
		description: 'Open or close this keyboard shortcut reference',
		code: 'Slash',
		keyLabel: '/',
		scope: 'global'
	}
];

export function isApplePlatform(platform: string): boolean {
	return /Mac|iPhone|iPad|iPod/i.test(platform);
}

export function platformModifierLabel(applePlatform: boolean): 'Cmd' | 'Ctrl' {
	return applePlatform ? 'Cmd' : 'Ctrl';
}

export function formatShortcut(command: ShortcutCommand, applePlatform: boolean): string {
	return `${platformModifierLabel(applePlatform)} + ${command.keyLabel}`;
}

export function ariaShortcut(command: ShortcutCommand, applePlatform: boolean): string {
	return `${applePlatform ? 'Meta' : 'Control'}+${command.keyLabel}`;
}

export function matchesShortcut(
	event: Pick<
		KeyboardEvent,
		| 'altKey'
		| 'code'
		| 'ctrlKey'
		| 'defaultPrevented'
		| 'isComposing'
		| 'metaKey'
		| 'repeat'
		| 'shiftKey'
	>,
	command: ShortcutCommand,
	applePlatform: boolean
): boolean {
	if (event.defaultPrevented || event.isComposing || event.repeat) return false;
	if (event.code !== command.code || event.altKey || event.shiftKey) return false;
	return applePlatform ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey;
}

export function findShortcutCommand(
	event: Parameters<typeof matchesShortcut>[0],
	applePlatform: boolean
): ShortcutCommand | undefined {
	return shortcutCommands.find((command) => matchesShortcut(event, command, applePlatform));
}

export function shortcutCommand(id: ShortcutCommandId): ShortcutCommand {
	const command = shortcutCommands.find((candidate) => candidate.id === id);
	if (!command) throw new Error(`Unknown shortcut command: ${id}`);
	return command;
}
