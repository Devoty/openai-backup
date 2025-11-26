import React, { useCallback } from "react";

function ConversationRow({ item, checked, active, onToggle, onPreview, previewLoading }) {
	const handleClick = useCallback(() => {
		onPreview(item.id);
	}, [item.id, onPreview]);

	const handleToggle = useCallback(
		(event) => {
			event.stopPropagation();
			onToggle(item.id);
		},
		[item.id, onToggle]
	);

	return (
		<div className={`conversation-item${active ? " active" : ""}`} onClick={handleClick}>
			<div className="conversation-left">
				<input type="checkbox" checked={checked} onChange={handleToggle} onClick={(e) => e.stopPropagation()} />
				<div className="conversation-main">
					<div className="conversation-title">{item.title || "(未命名对话)"}</div>
				</div>
			</div>
		</div>
	);
}

export default React.memo(ConversationRow);
