import React from "react";
import PreviewMessages from "./PreviewMessages";

const ConversationPreview = ({
	preview,
	handleBackToList,
	selectedCount,
	exportZipLoading,
	handleExportZip,
	importLoading,
	handleImport,
	singleDeleteLabel,
	handleSingleDelete,
	singleDeleteLoading,
}) => {
	return (
		<section className="panel preview-panel">
			<div className="preview-header">
				<div className="preview-title-group">
					<button type="button" className="ghost" onClick={handleBackToList}>
						返回
					</button>
					<div className="preview-title-wrap">
						<div className="preview-title">{preview.id ? preview.title || preview.id : "请选择左侧的对话查看详情"}</div>
						<div className="preview-subtitle">最近更新 {preview.updateTime || "-"}</div>
					</div>
				</div>
				<div className="preview-actions">
					<div className="button-group">
						<button type="button" className="secondary" onClick={handleExportZip} disabled={selectedCount === 0 || exportZipLoading}>
							导出 Markdown
						</button>
						<button type="button" className="secondary" onClick={() => handleImport("notion")} disabled={selectedCount === 0 || importLoading}>
							导出 Notion
						</button>
						<button type="button" className="secondary" onClick={() => handleImport("anytype")} disabled={selectedCount === 0 || importLoading}>
							导出 Anytype
						</button>
					</div>
					<button type="button" className="danger" onClick={handleSingleDelete} disabled={!preview.id || singleDeleteLoading}>
						{singleDeleteLabel}
					</button>
				</div>
			</div>
			<div className="preview-meta">
				<div className="meta-grid">
					<div className="meta-item">
						<span>对话 ID</span>
						<strong>{preview.id || "-"}</strong>
					</div>
					<div className="meta-item">
						<span>创建时间</span>
						<strong>{preview.createTime || "-"}</strong>
					</div>
					<div className="meta-item">
						<span>更新时间</span>
						<strong>{preview.updateTime || "-"}</strong>
					</div>
					<div className="meta-item">
						<span>来源</span>
						<strong>-</strong>
					</div>
				</div>
			</div>
			<div className="preview-content">
				<div className="message-wrapper">
					<PreviewMessages preview={preview} />
				</div>
			</div>
		</section>
	);
};

export default ConversationPreview;
