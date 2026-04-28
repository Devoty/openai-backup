import React from "react";
import GlobalToolbar from "./GlobalToolbar";
import ConversationList from "./ConversationList";
import ConversationPreview from "./ConversationPreview";

const Workspace = (props) => {
	return (
		<div className="workspace">
			<GlobalToolbar
				total={props.total}
				pageInfoText={props.pageInfoText}
				exportZipLabel={props.exportZipLabel}
				handleExportZip={props.handleExportZip}
				selectedCount={props.selectedCount}
				exportZipLoading={props.exportZipLoading}
				importLoading={props.importLoading}
				handleImport={props.handleImport}
				handleReload={props.handleReload}
				loading={props.loading}
				bulkDeleteLabel={props.bulkDeleteLabel}
				handleBulkDelete={props.handleBulkDelete}
				bulkDeleteLoading={props.bulkDeleteLoading}
			/>
			<main className="content-grid">
				<ConversationList
					totalLabel={props.totalLabel}
					targetHint={props.targetHint}
					searchTerm={props.searchTerm}
					handleSearchChange={props.handleSearchChange}
					target={props.target}
					handleTargetChange={props.handleTargetChange}
					limit={props.limit}
					handlePageSizeChange={props.handlePageSizeChange}
					loading={props.loading}
					filteredConversations={props.filteredConversations}
					selected={props.selected}
					preview={props.preview}
					toggleSelection={props.toggleSelection}
					handlePreview={props.handlePreview}
					pageInfoText={props.pageInfoText}
					handlePrevPage={props.handlePrevPage}
					canPrev={props.canPrev}
					handleNextPage={props.handleNextPage}
					canNext={props.canNext}
				/>
				<ConversationPreview
					preview={props.preview}
					handleBackToList={props.handleBackToList}
					selectedCount={props.selectedCount}
					exportZipLoading={props.exportZipLoading}
					handleExportZip={props.handleExportZip}
					importLoading={props.importLoading}
					handleImport={props.handleImport}
					singleDeleteLabel={props.singleDeleteLabel}
					handleSingleDelete={props.handleSingleDelete}
					singleDeleteLoading={props.singleDeleteLoading}
				/>
			</main>
		</div>
	);
};

export default Workspace;
