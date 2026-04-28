import React from "react";

const GlobalToolbar = ({
	total,
	pageInfoText,
	exportZipLabel,
	handleExportZip,
	selectedCount,
	exportZipLoading,
	importLoading,
	handleImport,
	handleReload,
	loading,
	bulkDeleteLabel,
	handleBulkDelete,
	bulkDeleteLoading,
}) => {
	return (
		<div className="global-toolbar">
			<div className="toolbar-left">
				<div className="toolbar-title">对话管理</div>
				<div className="toolbar-subtitle">
					<span>共 {total} 条</span>
					<span className="dot">•</span>
					<span>{pageInfoText}</span>
				</div>
			</div>
			<div className="toolbar-actions">
				<div className="button-group">
					<button type="button" onClick={handleExportZip} disabled={selectedCount === 0 || exportZipLoading}>
						{exportZipLabel}
					</button>
					<button
						type="button"
						className="secondary"
						onClick={() => handleImport("notion")}
						disabled={selectedCount === 0 || importLoading}
					>
						导出到 Notion
					</button>
					<button
						type="button"
						className="secondary"
						onClick={() => handleImport("anytype")}
						disabled={selectedCount === 0 || importLoading}
					>
						导出到 Anytype
					</button>
				</div>
				<div className="button-group ghost-group">
					<button type="button" className="ghost" onClick={handleReload} disabled={loading}>
						刷新
					</button>
					<button type="button" className="ghost danger-outline" onClick={handleBulkDelete} disabled={selectedCount === 0 || bulkDeleteLoading}>
						{bulkDeleteLabel}
					</button>
				</div>
			</div>
		</div>
	);
};

export default GlobalToolbar;
