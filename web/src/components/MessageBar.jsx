import React from "react";

function MessageBar({ message }) {
	const className = message.error ? "error" : message.text ? "info" : "";
	return (
		<div id="message" className={className}>
			{message.text}
		</div>
	);
}

export default React.memo(MessageBar);
