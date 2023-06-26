use axum::handler::HandlerWithoutStateExt;
use axum::http::StatusCode;
use axum::Router;
use tower_http::services::ServeDir;

pub fn routes(plugin_path: String) -> Router {
    async fn handle_404() -> (StatusCode, &'static str) {
        (StatusCode::NOT_FOUND, "Not Found")
    }
    let serve_dir =
        ServeDir::new(format!("{plugin_path}/web")).fallback(handle_404.into_service());
    return Router::new().nest_service("/", serve_dir);
}