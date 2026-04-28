import React from "react";
import ConfigForm from "./ConfigForm";

const SettingsPage = ({
	configDraft,
	configSections,
	handleConfigFieldChange,
	handleConfigSubmit,
	handleConfigReset,
	configSaving,
	configTab,
	handleConfigSectionChange,
	handleConfigImportClick,
	handleConfigExport,
	configImporting,
	configExporting,
	configImportInputRef,
	handleConfigImportFile,
}) => {
	return (
		<div className="settings-container">
			<div className="settings-meta-banner">分区配置导出路径与 Token，保存后立即生效。</div>
			<ConfigForm
				draft={configDraft}
				sections={configSections}
				onFieldChange={handleConfigFieldChange}
				onSubmit={handleConfigSubmit}
				onReset={handleConfigReset}
				saving={configSaving}
				activeSection={configTab}
				onSectionChange={handleConfigSectionChange}
				onImport={handleConfigImportClick}
				onExport={handleConfigExport}
				importing={configImporting}
				exporting={configExporting}
			/>
			<input
				ref={configImportInputRef}
				type="file"
				accept="application/json"
				style={{ display: "none" }}
				onChange={handleConfigImportFile}
			/>
		</div>
	);
};

export default SettingsPage;
