import React from "react";
import ConversationRow from "./ConversationRow";

const ConversationList = ({
	totalLabel,
	targetHint,
	searchTerm,
	handleSearchChange,
	target,
	handleTargetChange,
	limit,
	handlePageSizeChange,
	loading,
	filteredConversations,
	selected,
	preview,
	toggleSelection,
	handlePreview,
	pageInfoText,
	handlePrevPage,
	canPrev,
	handleNextPage,
	canNext,
}) => {
	return (
		<section className="panel list-panel">
			<div className="panel-header list-panel-header">
				<div className="list-heading">
					<h2>
						对话列表 <span>{totalLabel}</span>
					</h2>
					<div className="target-hint muted">{targetHint}</div>
				</div>
				<div className="list-tools">
					<div className="search-box">
						<input type="search" value={searchTerm} onChange={handleSearchChange} placeholder="搜索标题 / ID / 时间" />
					</div>
					<label className="inline-select">
						导出目标
						<select value={target} onChange={handleTargetChange}>
							<option value="anytype">Anytype</option>
							<option value="notion">Notion</option>
						</select>
					</label>
					<label className="page-size">
						每页
						<select value={limit} onChange={handlePageSizeChange}>
							<option value="10">10</option>
							<option value="20">20</option>
							<option value="50">50</option>
						</select>
					</label>
				</div>
			</div>
			<div className="list-body">
				{loading ? (
					<div className="empty-placeholder">正在加载…</div>
				) : filteredConversations.length === 0 ? (
					<div className="empty-placeholder">{searchTerm ? "没有匹配的对话" : "暂未获取到对话记录"}</div>
				) : (
					<div className="conversation-list">
						{filteredConversations.map((item) => (
							<ConversationRow
								key={item.id}
								item={item}
								checked={selected.has(item.id)}
								active={preview.id === item.id}
								onToggle={toggleSelection}
								onPreview={handlePreview}
								previewLoading={preview.loading && preview.id === item.id}
							/>
						))}
					</div>
				)}
			</div>
			<div className="pagination-bar">
				<button type="button" className="ghost" onClick={handlePrevPage} disabled={!canPrev}>
					上一页
				</button>
				<div className="page-info">{pageInfoText}</div>
				<button type="button" className="ghost" onClick={handleNextPage} disabled={!canNext}>
					下一页
				</button>
			</div>
		</section>
	);
};

export default ConversationList;
