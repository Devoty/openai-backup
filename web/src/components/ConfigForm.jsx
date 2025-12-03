import React, { useMemo, useState } from "react";

function ConfigField({ field, value, onChange }) {
	const { key, label, type = "text", placeholder, options = [], description, rows = 3, min, max, fullWidth, secureToggle } = field;
	const fieldClassName = fullWidth ? "form-field full-width" : "form-field";
	const fieldId = "config-" + key;
	const [visible, setVisible] = useState(false);

	if (type === "checkbox") {
		return (
			<div className={fieldClassName}>
				<label className="checkbox-row">
					<input
						id={fieldId}
						type="checkbox"
						checked={!!value}
						onChange={(event) => onChange(key, event.target.checked)}
					/>
					<span>{label}</span>
				</label>
				{description ? <div className="field-hint">{description}</div> : null}
			</div>
		);
	}

	if (type === "select") {
		return (
			<div className={fieldClassName}>
				<label htmlFor={fieldId}>{label}</label>
				<select id={fieldId} value={value == null ? "" : value} onChange={(event) => onChange(key, event.target.value)}>
					{options.map((option) => (
						<option key={option.value == null ? "" : option.value} value={option.value == null ? "" : option.value}>
							{option.label}
						</option>
					))}
				</select>
				{description ? <div className="field-hint">{description}</div> : null}
			</div>
		);
	}

	if (type === "password") {
		return (
			<div className={fieldClassName}>
				<label htmlFor={fieldId}>{label}</label>
				<div className="secure-input">
					<input
						id={fieldId}
						type={visible ? "text" : "password"}
						value={value == null ? "" : value}
						onChange={(event) => onChange(key, event.target.value)}
						placeholder={placeholder}
						autoComplete="off"
					/>
					{secureToggle ? (
						<button type="button" className="ghost-link" onClick={() => setVisible((prev) => !prev)}>
							{visible ? "隐藏" : "显示"}
						</button>
					) : null}
				</div>
				{description ? <div className="field-hint">{description}</div> : null}
			</div>
		);
	}

	if (type === "textarea") {
		return (
			<div className={fieldClassName}>
				<label htmlFor={fieldId}>{label}</label>
				<textarea
					id={fieldId}
					rows={rows || 3}
					value={value == null ? "" : value}
					onChange={(event) => onChange(key, event.target.value)}
					placeholder={placeholder}
				/>
				{description ? <div className="field-hint">{description}</div> : null}
			</div>
		);
	}

	return (
		<div className={fieldClassName}>
			<label htmlFor={fieldId}>{label}</label>
			<input
				id={fieldId}
				type={type}
				value={value == null ? "" : value}
				onChange={(event) => onChange(key, event.target.value)}
				placeholder={placeholder}
				min={min}
				max={max}
				inputMode={type === "number" ? "numeric" : undefined}
			/>
			{description ? <div className="field-hint">{description}</div> : null}
		</div>
	);
}

function ConfigSection({ section, draft, onFieldChange }) {
	return (
		<section className="settings-section">
			<h2>{section.title}</h2>
			{section.description ? <p>{section.description}</p> : null}
			<div className="settings-grid">
				{section.fields.map((field) => (
					<ConfigField key={field.key} field={field} value={draft[field.key]} onChange={onFieldChange} />
				))}
			</div>
		</section>
	);
}

function ConfigForm({
	draft,
	sections,
	onFieldChange,
	onSubmit,
	onReset,
	saving,
	activeSection,
	onSectionChange,
	onImport,
	onExport,
	importing,
	exporting
}) {
	const currentSection = useMemo(() => {
		if (!sections || sections.length === 0) {
			return null;
		}
		return sections.find((section) => section.key === activeSection) || sections[0];
	}, [activeSection, sections]);

	return (
	<form className="settings-form" onSubmit={onSubmit}>
		<div className="settings-tabs">
			<div className="tab-list">
				{(sections || []).map((section) => {
					const isActive = currentSection ? section.key === currentSection.key : false;
					return (
						<button
							key={section.key}
							type="button"
							className={isActive ? "active" : ""}
							onClick={() => (onSectionChange ? onSectionChange(section.key) : null)}
						>
							{section.title}
						</button>
					);
				})}
			</div>
			<div className="tab-actions">
				<button type="button" className="secondary" onClick={() => (onImport ? onImport() : null)} disabled={importing || saving}>
					{importing ? "导入中…" : "导入配置"}
				</button>
				<button type="button" onClick={() => (onExport ? onExport() : null)} disabled={exporting}>
					{exporting ? "导出中…" : "导出配置"}
				</button>
			</div>
		</div>
		{currentSection ? <ConfigSection section={currentSection} draft={draft} onFieldChange={onFieldChange} /> : null}
		<div className="form-actions sticky-footer">
			<div className="form-actions-left">
				<button type="button" className="secondary" onClick={onReset} disabled={saving}>
					重置修改
				</button>
			</div>
			<div className="form-actions-right">
				<button type="submit" disabled={saving}>
					{saving ? "保存中…" : "保存配置"}
				</button>
			</div>
		</div>
	</form>
);
}

export default React.memo(ConfigForm);
