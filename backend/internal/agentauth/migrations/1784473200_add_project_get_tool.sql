-- +migrate Up
ALTER TABLE agent_tokens
	DROP CONSTRAINT agent_tokens_tools_valid,
	ADD CONSTRAINT agent_tokens_tools_valid CHECK (
		allowed_tools <@ ARRAY[
			'task_get', 'task_search', 'project_get', 'project_list', 'view_query',
			'task_create', 'task_update', 'task_complete', 'task_reopen', 'task_move',
			'task_reorder'
		]::TEXT[]
	);

-- +migrate Down
ALTER TABLE agent_tokens
	DROP CONSTRAINT agent_tokens_tools_valid,
	ADD CONSTRAINT agent_tokens_tools_valid CHECK (
		allowed_tools <@ ARRAY[
			'task_get', 'task_search', 'project_list', 'view_query', 'task_create',
			'task_update', 'task_complete', 'task_reopen', 'task_move', 'task_reorder'
		]::TEXT[]
	);
