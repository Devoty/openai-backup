import React from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

function normalizeContent(value) {
	if (typeof value !== "string") {
		return "";
	}
	return value.replace(/\r\n/g, "\n").trim();
}

function MarkdownContent({ content }) {
	const normalized = normalizeContent(content || "");
	if (!normalized) {
		return null;
	}

	return (
		<div className="markdown-body">
			<ReactMarkdown
				remarkPlugins={[remarkGfm]}
				linkTarget="_blank"
				components={{
					code({ node, inline, className, children, ...props }) {
						const match = /language-([\w-]+)/.exec(className || "");
						if (inline) {
							return (
								<code className={className} {...props}>
									{children}
								</code>
							);
						}
						const lang = match ? match[1].toLowerCase() : "";
						return (
							<pre className={"code-block" + (lang ? " lang-" + lang : "")}>
								{lang ? <span className="code-lang">{lang}</span> : null}
								<code className={className} {...props}>
									{children}
								</code>
							</pre>
						);
					},
					a({ node, ...props }) {
						return <a {...props} target="_blank" rel="noopener noreferrer" />;
					}
				}}
			>
				{normalized}
			</ReactMarkdown>
		</div>
	);
}

export default MarkdownContent;
