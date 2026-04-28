import React from "react";

const Header = ({ listenLabel, timezoneLabel, activeTab, onOpenSettings }) => {
	return (
		<header className="app-header">
			<div className="brand">
				<div className="brand-logo">B</div>
				<div className="brand-text">
					<div className="brand-title">Backed</div>
					<div className="brand-subtitle">ChatGPT 对话整理与导出</div>
				</div>
			</div>
			<div className="brand-meta">
				<div className="pill">监听 {listenLabel || "-"}</div>
				<div className="pill">时区 {timezoneLabel || "-"}</div>
				<button
					type="button"
					className={`ghost ${activeTab === "settings" ? "active" : ""}`}
					onClick={onOpenSettings}
				>
					设置
				</button>
			</div>
		</header>
	);
};

export default Header;
