import React, { useCallback, useMemo } from "react";

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

	const timeLabel = useMemo(() => {
		return item.update_time || item.create_time || "-";
	}, [item.create_time, item.update_time]);

	const subtitle = useMemo(() => {
		if (item.create_time) {
			return "创建于 " + item.create_time;
		}
		return item.id || "";
	}, [item.create_time, item.id]);

	return (
		<div
			className={`conversation-item${active ? " selected" : ""}${checked ? " checked" : ""}`}
			onClick={handleClick}
			aria-pressed={active}
		>
			<div className="selection-bar" aria-hidden />
			<div className="conversation-left">
				<input type="checkbox" checked={checked} onChange={handleToggle} onClick={(e) => e.stopPropagation()} aria-label="选择对话" />
				<div className="conversation-main">
					<div className="conversation-title-row">
						<div className="conversation-title" title={item.title || "(未命名对话)"}>
							{item.title || "(未命名对话)"}
						</div>
						<div className="conversation-time">{previewLoading ? "加载中…" : timeLabel}</div>
					</div>
					<div className="conversation-subtitle" title={subtitle}>
						{subtitle}
					</div>
				</div>
			</div>
		</div>
	);
}

export default React.memo(ConversationRow);
