import { createGrpcWebTransport } from "@connectrpc/connect-web";

function resolveBaseUrl() {
	const raw = import.meta.env.VITE_GRPC_WEB_BASE_URL;
	if (typeof raw === "string" && raw.trim().length > 0) {
		return raw.trim().replace(/\/$/, "");
	}
	return "http://localhost:8081";
}

export const transport = createGrpcWebTransport({
	baseUrl: resolveBaseUrl(),
});
