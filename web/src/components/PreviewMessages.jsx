import React from "react";
import MarkdownContent from "./MarkdownContent";

function PreviewMessages({ preview }) {
	if (preview.loading) {
		return <div className="empty-placeholder">正在加载对话…</div>;
	}
	if (!preview.id) {
		return <div className="empty-placeholder">暂未选择对话。</div>;
	}
	if (!preview.messages || preview.messages.length === 0) {
		return <div className="empty-placeholder">这条对话没有可展示的消息。</div>;
	}
	return (
		<div className="message-list">
			{preview.messages.map((msg, index) => {
				const role = msg.role ? msg.role.toLowerCase() : "unknown";
				const label = msg.role ? msg.role.toUpperCase() : "UNKNOWN";
				return (
					<div className={"message role-" + role} key={preview.id + "-" + index}>
						<div className="message-header">
							<span className="message-role">{label}</span>
							<span className="message-time">{msg.timestamp || "-"}</span>
						</div>
						<div className="message-body">
							<MarkdownContent content={msg.text || ""} />
						</div>
						{Array.isArray(msg.references) && msg.references.length > 0 ? (
							<div className="message-references">
								<div className="references-title">引用 / 来源</div>
								<ul>
									{msg.references.map((ref, refIdx) => (
										<li key={"ref-" + index + "-" + refIdx}>
											<a href={ref.url} target="_blank" rel="noopener noreferrer">
												{ref.title || ref.url}
											</a>
											{ref.source ? <span className="reference-source">{ref.source}</span> : null}
										</li>
									))}
								</ul>
							</div>
						) : null}
					</div>
				);
			})}
		</div>
	);
}

export default React.memo(PreviewMessages);
