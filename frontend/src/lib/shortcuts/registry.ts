export type ProductCommandId =
	| 'command-palette'
	| 'quick-add'
	| 'toggle-chat'
	| 'project-overview'
	| 'project-inbox'
	| 'project-today'
	| 'project-tasks'
	| 'project-activity'
	| 'project-settings'
	| 'manage-projects'
	| 'account-settings'
	| 'toggle-help';

export interface CommandShortcut {
	code: string;
	keyLabel: string;
	allowAlt?: boolean;
}

export interface ProductCommand {
	id: ProductCommandId;
	label: string;
	description: string;
	aliases: readonly string[];
	scope: 'global' | 'project';
	shortcut?: CommandShortcut;
}

export interface ShortcutCommand extends ProductCommand {
	shortcut: CommandShortcut;
	code: string;
	keyLabel: string;
}

export const commandRegistry: readonly ProductCommand[] = [
	{
		id: 'command-palette',
		label: 'Command palette',
		description: 'Search actions, projects, and tasks',
		aliases: ['search', 'jump', 'go to'],
		scope: 'global',
		shortcut: { code: 'KeyK', keyLabel: 'K' }
	},
	{
		id: 'quick-add',
		label: 'Quick add',
		description: 'Create a task from anywhere in the active project',
		aliases: ['create task', 'new task'],
		scope: 'project',
		shortcut: { code: 'KeyN', keyLabel: 'N', allowAlt: true }
	},
	{
		id: 'toggle-chat',
		label: 'Assistant',
		description: 'Open or close the active project assistant',
		aliases: ['chat', 'agent'],
		scope: 'project',
		shortcut: { code: 'KeyJ', keyLabel: 'J' }
	},
	{
		id: 'project-overview',
		label: 'Project overview',
		description: 'Open the active project overview',
		aliases: ['overview', 'dashboard'],
		scope: 'project',
		shortcut: { code: 'Digit1', keyLabel: '1' }
	},
	{
		id: 'project-inbox',
		label: 'Inbox',
		description: 'Open the active project Inbox',
		aliases: ['unsorted'],
		scope: 'project',
		shortcut: { code: 'Digit2', keyLabel: '2' }
	},
	{
		id: 'project-today',
		label: 'Today',
		description: 'Open the active project Today view',
		aliases: ['due today'],
		scope: 'project',
		shortcut: { code: 'Digit3', keyLabel: '3' }
	},
	{
		id: 'project-tasks',
		label: 'Tasks',
		description: 'Open all tasks in the active project',
		aliases: ['all tasks'],
		scope: 'project',
		shortcut: { code: 'Digit4', keyLabel: '4' }
	},
	{
		id: 'project-activity',
		label: 'Activity',
		description: 'Open the active project activity feed',
		aliases: ['history', 'events'],
		scope: 'project',
		shortcut: { code: 'Digit5', keyLabel: '5' }
	},
	{
		id: 'project-settings',
		label: 'Project settings',
		description: 'Configure the active project',
		aliases: ['workspace settings'],
		scope: 'project',
		shortcut: { code: 'Digit6', keyLabel: '6' }
	},
	{
		id: 'manage-projects',
		label: 'Manage projects',
		description: 'Create, organize, or archive projects',
		aliases: ['projects', 'workspaces'],
		scope: 'global'
	},
	{
		id: 'account-settings',
		label: 'Account settings',
		description: 'Configure your account and agent defaults',
		aliases: ['settings', 'profile'],
		scope: 'global'
	},
	{
		id: 'toggle-help',
		label: 'Keyboard shortcuts',
		description: 'Open or close the keyboard shortcut reference',
		aliases: ['help', 'hotkeys'],
		scope: 'global',
		shortcut: { code: 'Slash', keyLabel: '/' }
	}
];

export const shortcutCommands: readonly ShortcutCommand[] = commandRegistry
	.filter((command) => command.shortcut !== undefined)
	.map((command) => ({
		...command,
		shortcut: command.shortcut!,
		code: command.shortcut!.code,
		keyLabel: command.shortcut!.keyLabel
	}));

export function isApplePlatform(platform: string): boolean {
	return /Mac|iPhone|iPad|iPod/i.test(platform);
}

export function platformModifierLabel(applePlatform: boolean): 'Cmd' | 'Ctrl' {
	return applePlatform ? 'Cmd' : 'Ctrl';
}

export function formatShortcut(command: ShortcutCommand, applePlatform: boolean): string {
	return `${platformModifierLabel(applePlatform)} + ${command.keyLabel}`;
}

export function formatShortcuts(command: ShortcutCommand, applePlatform: boolean): string[] {
	const shortcuts = [formatShortcut(command, applePlatform)];
	if (command.shortcut.allowAlt) {
		shortcuts.push(
			`${platformModifierLabel(applePlatform)} + ${applePlatform ? 'Option' : 'Alt'} + ${command.keyLabel}`
		);
	}
	return shortcuts;
}

export function formatShortcutHint(command: ShortcutCommand, applePlatform: boolean): string {
	if (!command.shortcut.allowAlt) return command.keyLabel;
	return `${command.keyLabel} / ${applePlatform ? 'Option' : 'Alt'} + ${command.keyLabel}`;
}

export function ariaShortcut(command: ShortcutCommand, applePlatform: boolean): string {
	const modifier = applePlatform ? 'Meta' : 'Control';
	const shortcuts = [`${modifier}+${command.keyLabel}`];
	if (command.shortcut.allowAlt) shortcuts.push(`${modifier}+Alt+${command.keyLabel}`);
	return shortcuts.join(' ');
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
	if (event.code !== command.code || event.shiftKey) return false;
	if (event.altKey && !command.shortcut.allowAlt) return false;
	return applePlatform ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey;
}

export function findShortcutCommand(
	event: Parameters<typeof matchesShortcut>[0],
	applePlatform: boolean
): ShortcutCommand | undefined {
	return shortcutCommands.find((command) => matchesShortcut(event, command, applePlatform));
}

export function shortcutCommand(id: ProductCommandId): ShortcutCommand {
	const command = shortcutCommands.find((candidate) => candidate.id === id);
	if (!command) throw new Error(`Command has no shortcut: ${id}`);
	return command;
}
